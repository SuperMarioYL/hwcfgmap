package profile

// This file is the 信创 (xinchuang / domestic-GPU) vendor-CLI probing seam —
// the m3_cover_cn_cards delivery.
//
// Probing is grounded in captured real output, never guessed:
//
//   - huawei (昇腾 / Ascend): `npu-smi info` table parsing — NPU index, model
//     name, and the Memory-Usage(MB)/HBM-Usage(MB) "used / total" MiB pools —
//     pinned against captured 910B1 (64 GiB HBM per NPU) and 910PremiumA
//     (32 GiB HBM) outputs from npu-smi 23.0.rc2[.2]; 1Panel and AIMA ship
//     parsers for the same `npu-smi info` interface.
//   - moorethreads (摩尔线程): `mthreads-gmi -q -j` JSON (gpus[].product_name +
//     memory_total) when the query CLI is present — the same interface AIMA
//     parses and sglang reads FB memory through; otherwise bare `mthreads-smi`
//     presence detection.
//   - biren (壁仞): presence-only — no captured bre-smi output or public
//     format spec exists to verify a parser against.
//
// The rule throughout: a documented zero is honest, a fabricated number is
// the bug that breaks a 信创 ops box. Every parser degrades to the presence
// stub exactly when a shape is unrecognized — never a guessed VRAM.

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CNVendorCLI maps a 信创 vendor to the presence CLI binary name we exec to
// detect it. All are optional on the box — presence implies the card family.
// (moorethreads additionally has a query CLI, mthreads-gmi, preferred by
// probeMooreThreads when present.)
var CNVendorCLI = map[string]string{
	VendorHuawei:       "npu-smi",      // 昇腾 / Ascend
	VendorMooreThreads: "mthreads-smi", // 摩尔线程 / Moore Threads
	VendorBiren:        "bre-smi",      // 壁仞 / Biren (best-effort name)
}

// CNVendorBackend maps a 信创 vendor to the llama.cpp backend its card maps to.
var CNVendorBackend = map[string]string{
	VendorHuawei:       BackendCANN,
	VendorMooreThreads: BackendMUSA,
	VendorBiren:        BackendUnknown, // 壁仞 backend not yet mapped upstream
}

// ProbeCNCards runs each 信创 vendor CLI present on PATH and reports the
// detected cards with as much detail as the CLI's documented output allows.
// On a host with no vendor CLI (a dev Mac, a CI runner, a pure-CPU box) the
// returned slice is empty — the synthesizer then emits a CPU-only launch
// line. Detection never fails the probe.
func ProbeCNCards() ([]GPUCard, error) {
	var cards []GPUCard
	if c, err := probeAscend(); err == nil {
		cards = append(cards, c...)
	}
	if c, err := probeMooreThreads(); err == nil {
		cards = append(cards, c...)
	}
	if c, err := probeBiren(); err == nil {
		cards = append(cards, c...)
	}
	return cards, nil
}

// probeAscend runs `npu-smi info` and parses the documented table. A CLI that
// runs but prints an unrecognized table degrades to the presence stub with an
// honest zero VRAM.
func probeAscend() ([]GPUCard, error) {
	cli := CNVendorCLI[VendorHuawei]
	bin, err := exec.LookPath(cli)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(bin, "info").Output()
	if err != nil {
		return nil, err
	}
	if cards := parseNPUSMITable(string(out)); len(cards) > 0 {
		return cards, nil
	}
	return []GPUCard{cnStubCard(VendorHuawei, cli)}, nil
}

// npuMemPair matches one "used / total" MiB pool inside a chip row
// (e.g. "0    / 0          65099/ 65536" → (0,0) and (65099,65536)).
var npuMemPair = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)

// parseNPUSMITable parses the `npu-smi info` card table into one GPUCard per
// NPU block. Layout (captured 910B1 / 910PremiumA, npu-smi 23.0.rc2[.2]):
//
//	| NPU   Name        | Health | Power(W)  Temp(C)  Hugepages-Usage(page)|
//	| Chip              | Bus-Id | AICore(%) Memory-Usage(MB)  HBM-Usage(MB)|
//	| 0     910B1       | OK     | 271.1     41          0    / 0           |
//	| 0                 | 0000:C1:00.0 | 55    0    / 0     65099/ 65536    |
//
// VRAM is the largest reported pool total on the chip row — HBM on 910B1
// (Memory-Usage reports 0/0) and 910PremiumA (32768 MiB HBM over the 15137
// MiB Memory pool). Blocks that do not match this shape are skipped; a
// trailing per-process table ("Process id" section) is not hardware topology
// and stops the parse.
func parseNPUSMITable(out string) []GPUCard {
	var cards []GPUCard
	type pendingNPU struct {
		index int
		name  string
	}
	var pending *pendingNPU
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Process id") {
			break
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitTableCells(trimmed)
		if len(cells) < 2 {
			continue
		}
		first := strings.Fields(cells[0])
		if len(first) >= 2 {
			// NPU row: "0  910B1" — an integer index followed by a model
			// name. Per-process rows are "npu chip" (two integers) and are
			// excluded by requiring the name to be non-numeric.
			if idx, err := strconv.Atoi(first[0]); err == nil {
				name := strings.Join(first[1:], " ")
				if name != "" && !isAllDigits(name) {
					pending = &pendingNPU{index: idx, name: name}
				}
			}
			continue
		}
		// Chip row: a bare integer first cell completes the pending NPU with
		// the memory pools found across the rest of the row.
		if pending != nil && len(first) == 1 {
			if _, err := strconv.Atoi(first[0]); err == nil {
				var vramMiB uint64
				for _, m := range npuMemPair.FindAllStringSubmatch(strings.Join(cells[1:], " "), -1) {
					if total, err := strconv.ParseUint(m[2], 10, 64); err == nil && total > vramMiB {
						vramMiB = total
					}
				}
				cards = append(cards, GPUCard{
					Vendor:    VendorHuawei,
					Model:     pending.name,
					VRAMBytes: vramMiB * 1024 * 1024,
					Backend:   CNVendorBackend[VendorHuawei],
					Index:     pending.index,
					Source:    CNVendorCLI[VendorHuawei],
				})
				pending = nil
			}
		}
	}
	return cards
}

// probeMooreThreads detects Moore Threads cards. mthreads-gmi is the
// machine-readable query CLI on MTT S80/S4000-class boxes; when it is absent
// or its output is unparseable, fall back to bare mthreads-smi presence
// detection — the card is still reported with backend musa and an honest
// zero VRAM.
func probeMooreThreads() ([]GPUCard, error) {
	if bin, err := exec.LookPath("mthreads-gmi"); err == nil {
		if out, err := exec.Command(bin, "-q", "-j").Output(); err == nil {
			if cards := parseMThreadsGMI(out); len(cards) > 0 {
				return cards, nil
			}
		}
	}
	cli := CNVendorCLI[VendorMooreThreads]
	bin, err := exec.LookPath(cli)
	if err != nil {
		return nil, err
	}
	if err := exec.Command(bin).Run(); err != nil {
		return nil, err
	}
	return []GPUCard{cnStubCard(VendorMooreThreads, cli)}, nil
}

// parseMThreadsGMI parses the documented `mthreads-gmi -q -j` shape:
// {"gpus": [{"product_name": "MTT S80", "memory_total": "16384 MiB", ...}]}.
// Returns nil on any other shape — the caller falls back to presence
// detection rather than guessing.
func parseMThreadsGMI(data []byte) []GPUCard {
	var doc struct {
		GPUs []struct {
			ProductName string `json:"product_name"`
			MemoryTotal string `json:"memory_total"`
		} `json:"gpus"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var cards []GPUCard
	for i, g := range doc.GPUs {
		if g.ProductName == "" {
			continue
		}
		cards = append(cards, GPUCard{
			Vendor:    VendorMooreThreads,
			Model:     g.ProductName,
			VRAMBytes: parseMiB(g.MemoryTotal) * 1024 * 1024,
			Backend:   CNVendorBackend[VendorMooreThreads],
			Index:     i,
			Source:    "mthreads-gmi",
		})
	}
	return cards
}

// parseMiB extracts the leading integer from value strings like "16384 MiB".
func parseMiB(s string) uint64 {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// probeBiren is presence-only: no captured bre-smi output or public format
// spec exists to verify a parser against, so the card is reported with an
// honest zero VRAM.
func probeBiren() ([]GPUCard, error) {
	cli := CNVendorCLI[VendorBiren]
	bin, err := exec.LookPath(cli)
	if err != nil {
		return nil, err
	}
	if err := exec.Command(bin, "info").Run(); err != nil {
		return nil, err
	}
	return []GPUCard{cnStubCard(VendorBiren, cli)}, nil
}

// cnStubCard is the presence-only fallback: the vendor CLI is on PATH and
// runs, but its output was not parsed (unrecognized shape, or a vendor with
// no verifiable format) — the card is still reported with VRAM 0 so the
// synthesizer picks the right backend suffix and flags the box as 信创.
func cnStubCard(vendor, source string) GPUCard {
	return GPUCard{
		Vendor:    vendor,
		Model:     vendorModelDefault(vendor),
		VRAMBytes: 0,
		Backend:   CNVendorBackend[vendor],
		Index:     0,
		Source:    source,
	}
}

// vendorModelDefault is the stand-in marketing name used until the real model
// string is parsed out of the vendor CLI output.
func vendorModelDefault(vendor string) string {
	switch vendor {
	case VendorHuawei:
		return "Ascend NPU (model not reported)"
	case VendorMooreThreads:
		return "Moore Threads GPU (model not reported)"
	case VendorBiren:
		return "Biren GPU (model not reported)"
	default:
		return strings.Join([]string{vendor, "(unknown)"}, " ")
	}
}

// splitTableCells splits one `| a | b | c |` table row into its trimmed cells.
func splitTableCells(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// isAllDigits reports whether s is a non-empty string of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

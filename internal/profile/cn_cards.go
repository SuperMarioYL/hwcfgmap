package profile

// This file is the 信创 (xinchuang / domestic-GPU) vendor-CLI probing seam.
//
// m1 contract (per mvp_plan §5 m1_probe_hardware): "信创卡 vendor CLI 探测框架
// 就位（先 stub）". So this file lands the framework — the function signature
// the probe package calls, the vendor→backend mapping, and a best-effort exec
// of the vendor CLIs — but the per-vendor parsing of npu-smi / mthreads-smi /
// bre-smi output is intentionally a stub: if the CLI is present we record the
// card as detected-vendor with VRAMBytes=0 and Backend set, so the synthesizer
// can still produce a CANN/MUSA-aware launch line; full VRAM parsing lands in
// m3_cover_cn_cards.
//
// Why a stub now: on a developer macOS host none of these CLIs exist, and the
// schema assumptions (npu-smi/mthreads-smi exact JSON shape) are flagged
// UNVERIFIED in the plan. We refuse to guess at VRAM numbers we have not seen
// on a real card — a documented zero is honest, a fabricated number is the
// bug that breaks a 信创 ops box.

import (
	"os/exec"
	"strings"
)

// CNVendorCLI maps a 信创 vendor to the CLI binary name we exec to detect it.
// All three are optional on the box — presence implies the card family.
var CNVendorCLI = map[string]string{
	VendorHuawei:       "npu-smi",       // 昇腾 / Ascend
	VendorMooreThreads: "mthreads-smi",  // 摩尔线程 / Moore Threads
	VendorBiren:        "bre-smi",       // 壁仞 / Biren (best-effort name)
}

// CNVendorBackend maps a 信创 vendor to the llama.cpp backend its card maps to.
var CNVendorBackend = map[string]string{
	VendorHuawei:       BackendCANN,
	VendorMooreThreads: BackendMUSA,
	VendorBiren:        BackendUnknown, // 壁仞 backend not yet mapped upstream
}

// ProbeCNCards runs each 信创 vendor CLI that is present on PATH and reports
// one GPUCard per detected CLI. m1: VRAM is left at 0 (parsing lands in m3);
// the card is still reported so the synthesizer picks the right backend
// suffix and flags the box as 信创. Returns an empty slice (not an error) when
// no domestic CLI is installed — the common case on dev/CI hosts.
func ProbeCNCards() ([]GPUCard, error) {
	var cards []GPUCard
	for vendor, cli := range CNVendorCLI {
		path, err := exec.LookPath(cli)
		if err != nil {
			continue // CLI not installed → card family not present on this box
		}
		// m1: invoke to confirm the CLI actually runs (some boxes ship the
		// binary but no driver). We do not parse VRAM yet — see file header.
		cmd := exec.Command(path, "info")
		if err := cmd.Run(); err != nil {
			continue
		}
		cards = append(cards, GPUCard{
			Vendor:    vendor,
			Model:     vendorModelDefault(vendor),
			VRAMBytes: 0, // m3 will parse npu-smi/mthreads-smi VRAM here
			Backend:   CNVendorBackend[vendor],
			Index:     0,
			Source:    cli,
		})
	}
	return cards, nil
}

// vendorModelDefault is the stand-in marketing name used until m3 parses the
// real model string out of the vendor CLI output.
func vendorModelDefault(vendor string) string {
	switch vendor {
	case VendorHuawei:
		return "Ascend NPU (model parsed in m3)"
	case VendorMooreThreads:
		return "Moore Threads GPU (model parsed in m3)"
	case VendorBiren:
		return "Biren GPU (model parsed in m3)"
	default:
		return strings.Join([]string{vendor, "(unknown)"}, " ")
	}
}

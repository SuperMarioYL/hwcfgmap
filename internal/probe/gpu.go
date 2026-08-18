package probe

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"

	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
)

// ProbeGPU assembles the GPU list from every vendor CLI present on PATH:
// nvidia-smi (CUDA), rocm-smi (ROCm), and the 信创 vendor CLIs (CANN/MUSA,
// via profile.ProbeCNCards — m1 stub, full VRAM in m3). Cards are de-duped by
// (vendor, index). On a host with no vendor CLI (a dev Mac, a CI runner, a
// pure-CPU box) the returned slice is empty and nil — the synthesizer then
// emits a CPU-only launch line. Detection never fails the probe.
func ProbeGPU() ([]profile.GPUCard, error) {
	var cards []profile.GPUCard

	if nv, err := probeNVIDIA(); err == nil {
		cards = append(cards, nv...)
	}
	if amd, err := probeROCm(); err == nil {
		cards = append(cards, amd...)
	}
	if cn, err := profile.ProbeCNCards(); err == nil {
		cards = append(cards, cn...)
	}
	return dedupe(cards), nil
}

// probeNVIDIA runs `nvidia-smi --query-gpu=index,name,memory.total
// --format=csv,noheader,nounits` and parses one card per line. The
// memory.total column is in MiB (nounits); we convert to bytes. This is the
// stable, documented nvidia-smi query interface — VRAM is fully parsed here
// (the m1 plan groups CUDA with "走通").
func probeNVIDIA() ([]profile.GPUCard, error) {
	bin, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(bin,
		"--query-gpu=index,name,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil, err
	}
	var cards []profile.GPUCard
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		name := strings.TrimSpace(strings.Join(fields[1:len(fields)-1], ","))
		mib, _ := strconv.ParseUint(strings.TrimSpace(fields[len(fields)-1]), 10, 64)
		cards = append(cards, profile.GPUCard{
			Vendor:    profile.VendorNVIDIA,
			Model:     name,
			VRAMBytes: mib * 1024 * 1024,
			Backend:   profile.BackendCUDA,
			Index:     idx,
			Source:    "nvidia-smi",
		})
	}
	return cards, nil
}

// probeROCm runs `rocm-smi --showmeminfo vram --json` and parses the documented
// ROCm 5.x/6.x shape: { "cardN": { "VRAM": { "Total VRAM": "<bytes>" } } }.
// Total VRAM is in bytes (no unit conversion). If the installed rocm-smi emits
// a different JSON shape, the card is still reported with backend=rocm and
// VRAMBytes=0 — detection succeeds, the unparseable VRAM is surfaced honestly
// rather than guessed. Full cross-version VRAM parsing lands in m3.
func probeROCm() ([]profile.GPUCard, error) {
	bin, err := exec.LookPath("rocm-smi")
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(bin, "--showmeminfo", "vram", "--json").Output()
	if err != nil {
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(out, &top); err != nil {
		return nil, err
	}
	var cards []profile.GPUCard
	// ROCm enumerates cards as card0..cardN in insertion order; sort-stable below.
	for cardKey, raw := range top {
		idx := rocmIndex(cardKey)
		vram := rocmTotalVRAM(raw) // 0 if shape differs — honest, not guessed
		cards = append(cards, profile.GPUCard{
			Vendor:    profile.VendorAMD,
			Model:     "AMD ROCm GPU",
			VRAMBytes: vram,
			Backend:   profile.BackendROCm,
			Index:     idx,
			Source:    "rocm-smi",
		})
	}
	// stable order by index
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0 && cards[j].Index < cards[j-1].Index; j-- {
			cards[j], cards[j-1] = cards[j-1], cards[j]
		}
	}
	return cards, nil
}

// rocmIndex parses the trailing integer of a "cardN" key.
func rocmIndex(key string) int {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.TrimPrefix(key, "card")
	n, _ := strconv.Atoi(key)
	return n
}

// rocmTotalVRAM walks one card's JSON for the documented "VRAM"."Total VRAM"
// bytes string. Returns 0 if the shape is not the documented one — the caller
// reports the card with VRAMBytes=0 so the synthesizer still selects backend=rocm.
func rocmTotalVRAM(raw json.RawMessage) uint64 {
	var mem map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mem); err != nil {
		return 0
	}
	vram, ok := mem["VRAM"]
	if !ok {
		// tolerant: some builds nest under "Memory"
		vram, ok = mem["Memory"]
		if !ok {
			return 0
		}
	}
	var fields map[string]string
	if err := json.Unmarshal(vram, &fields); err != nil {
		return 0
	}
	for k, v := range fields {
		if strings.Contains(strings.ToLower(k), "total") && strings.Contains(strings.ToLower(k), "vram") {
			n, _ := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
			return n
		}
	}
	return 0
}

// dedupe collapses cards that two detectors both reported (same vendor+index),
// keeping the one with a non-zero VRAM when available.
func dedupe(cards []profile.GPUCard) []profile.GPUCard {
	seen := make(map[string]profile.GPUCard, len(cards))
	order := []string{}
	for _, c := range cards {
		key := c.Vendor + ":" + strconv.Itoa(c.Index)
		prev, ok := seen[key]
		if !ok {
			seen[key] = c
			order = append(order, key)
			continue
		}
		if c.VRAMBytes > prev.VRAMBytes {
			seen[key] = c
		}
	}
	out := make([]profile.GPUCard, 0, len(order))
	for _, k := range order {
		out = append(out, seen[k])
	}
	return out
}

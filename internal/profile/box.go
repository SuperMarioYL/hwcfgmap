// Package profile holds the hardware BoxProfile — the probe output that the
// synthesizer consumes. It is the shared data model between internal/probe
// (producer) and internal/synth (consumer).
package profile

// Backend enumerates the llama.cpp compute backend a card maps to.
const (
	BackendCUDA  = "cuda"
	BackendROCm  = "rocm"
	BackendCANN  = "cann"  // 昇腾 / Ascend NPU
	BackendMUSA  = "musa"  // 摩尔线程 / Moore Threads
	BackendCPU   = "cpu"
	BackendUnknown = "unknown"
)

// Vendor enumerates GPU vendors we can detect from the box's vendor CLIs.
const (
	VendorNVIDIA      = "nvidia"
	VendorAMD         = "amd"
	VendorHuawei      = "huawei"       // 昇腾
	VendorMooreThreads = "moorethreads" // 摩尔线程
	VendorBiren       = "biren"        // 壁仞
	VendorUnknown     = "unknown"
)

// GPUCard describes one compute card seen on the box.
type GPUCard struct {
	// Vendor is the detected vendor (nvidia/amd/huawei/moorethreads/biren/unknown).
	Vendor string `json:"vendor"`
	// Model is the marketing name (e.g. "NVIDIA GeForce RTX 4090", "Ascend 910B").
	Model string `json:"model"`
	// VRAMBytes is the device memory in bytes (0 if the vendor CLI did not report).
	VRAMBytes uint64 `json:"vram_bytes"`
	// Backend is the llama.cpp backend this card maps to (cuda/rocm/cann/musa/cpu).
	Backend string `json:"backend"`
	// Index is the device index as reported by the vendor CLI (0-based).
	Index int `json:"index"`
	// Source is how the card was detected ("nvidia-smi", "rocm-smi", "npu-smi",
	// "mthreads-smi", "fallback"). Pure observability — the synthesizer does
	// not branch on it.
	Source string `json:"source,omitempty"`
}

// CPUInfo summarises the host CPU topology the synthesizer cares about.
type CPUInfo struct {
	// PhysicalCores is the number of physical cores (best-effort; on hosts where
	// gopsutil cannot separate physical from logical, equals Threads/2).
	PhysicalCores int `json:"physical_cores"`
	// Threads is the logical processor count.
	Threads int `json:"threads"`
	// NUMANodes is the number of NUMA nodes (0 when not reported).
	NUMANodes int `json:"numa_nodes"`
	// Model is the CPU model name.
	Model string `json:"model"`
}

// DiskInfo is a best-effort NVMe IO summary. It is NOT required for ArgMatrix
// synthesis — the synthesizer keys off VRAM/RAM/cores. It ships in the
// BoxProfile so operators can eyeball IO headroom.
type DiskInfo struct {
	// Name is the device reported by gopsutil (e.g. "nvme0n1").
	Name string `json:"name,omitempty"`
	// ReadBytes is the cumulative bytes read since boot, straight from gopsutil
	// disk.IOCounters. It is an observability signal, NOT a throughput rate —
	// a real seq-read rate needs the nvme-cli datasheet or a sampling window,
	// both out of scope for m1.
	ReadBytes uint64 `json:"read_bytes,omitempty"`
}

// BoxProfile is the complete hardware picture of one box: GPU(s) + CPU + RAM +
// an optional NVMe summary. Probe produces it; Synthesize consumes it. The
// JSON shape is the stable m1 contract — `hwcfgmap probe` emits this verbatim.
type BoxProfile struct {
	GPU  []GPUCard `json:"gpu"`
	CPU  CPUInfo   `json:"cpu"`
	RAM  uint64    `json:"ram_bytes"`
	NVMe *DiskInfo `json:"nvme,omitempty"`
}

// TotalVRAM returns the cumulative device memory across all detected cards, in
// bytes. Zero on a CPU-only box.
func (b BoxProfile) TotalVRAM() uint64 {
	var sum uint64
	for _, g := range b.GPU {
		sum += g.VRAMBytes
	}
	return sum
}

// HasGPU reports whether any compute card was detected.
func (b BoxProfile) HasGPU() bool { return len(b.GPU) > 0 }

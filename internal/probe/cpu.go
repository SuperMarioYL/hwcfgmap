// Package probe reads the box — GPU (incl. 信创 vendor CLIs), CPU, RAM, NVMe —
// and assembles a profile.BoxProfile. It is the producer side of the
// probe-and-emit primitive; internal/synth is the consumer.
package probe

import (
	"github.com/shirou/gopsutil/v4/cpu"

	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
)

// ProbeCPU reads the host CPU topology via gopsutil. It never returns an error
// for "could not read" — it degrades to the best value available (physical 1,
// threads = physical) so a constrained host still gets a usable BoxProfile.
// The only error path is gopsutil being entirely unavailable.
func ProbeCPU() (profile.CPUInfo, error) {
	info := profile.CPUInfo{}

	physical, perr := cpu.Counts(false) // physical cores
	logical, lerr := cpu.Counts(true)    // logical processors (SMT)
	switch {
	case perr == nil && lerr == nil:
		info.PhysicalCores = physical
		info.Threads = logical
	case lerr == nil:
		// physical unknown — derive from logical assuming SMT×2.
		info.Threads = logical
		info.PhysicalCores = logical / 2
	case perr == nil:
		info.PhysicalCores = physical
		info.Threads = physical
	default:
		info.PhysicalCores = 1
		info.Threads = 1
	}
	if info.PhysicalCores <= 0 {
		info.PhysicalCores = 1
	}
	if info.Threads < info.PhysicalCores {
		info.Threads = info.PhysicalCores
	}

	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		info.Model = infos[0].ModelName
	}
	// gopsutil does not expose NUMA node count directly; leave 0 (the
	// synthesizer does not branch on it — m1 maps threads to physical cores).
	return info, nil
}

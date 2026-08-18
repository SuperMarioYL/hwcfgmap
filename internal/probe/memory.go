package probe

import (
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
)

// ProbeMemory reads total RAM via gopsutil, and best-effort attaches the most
// active NVMe device's cumulative read counter. The NVMe summary is purely
// observability — it is NOT consumed by the synthesizer and is omitted
// entirely when no nvme* device is reported.
func ProbeMemory() (ram uint64, nvme *profile.DiskInfo, err error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, nil, err
	}
	ram = vm.Total

	counters, derr := disk.IOCounters()
	if derr != nil {
		return ram, nil, nil // disk IO is best-effort; never fail the probe on it
	}
	var bestName string
	var bestRead uint64
	for name, c := range counters {
		if !strings.HasPrefix(strings.ToLower(name), "nvme") {
			continue
		}
		// pick the nvme device with the most cumulative read activity
		if c.ReadBytes > bestRead {
			bestRead = c.ReadBytes
			bestName = name
		}
	}
	if bestName != "" {
		nvme = &profile.DiskInfo{Name: bestName, ReadBytes: bestRead}
	}
	return ram, nvme, nil
}

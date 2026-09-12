package synth

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/hwcfgmap/internal/modeltargets"
	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
)

const (
	gib = uint64(1024 * 1024 * 1024)
	mib = uint64(1024 * 1024)
)

func gpuCard(vramBytes uint64) profile.GPUCard {
	return profile.GPUCard{
		Vendor:    profile.VendorNVIDIA,
		Model:     "test GPU",
		VRAMBytes: vramBytes,
		Backend:   profile.BackendCUDA,
		Index:     0,
	}
}

// CPU-only box (no vendor CLI on a dev Mac / CI runner): offload nothing, cap
// context by 0.8×RAM, and mlock when RAM holds the full q4 weights.
func TestSynthesize_CPUOnly(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: nil,
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 32 * gib,
	}
	am := Synthesize(bp, modeltargets.Qwen3_27B)

	if am.NGPULayers != 0 {
		t.Fatalf("cpu-only box: want nGpu=0, got %d", am.NGPULayers)
	}
	if am.Threads != 8 {
		t.Fatalf("cpu-only box: want threads=physical cores (8), got %d", am.Threads)
	}
	// 0.8*32GiB / (2048*64) = 25.7GiB / 131072 ≈ 205k tokens → 8192 (recommended) wins.
	if am.ContextSize != 8192 {
		t.Fatalf("cpu-only 32GiB box: want ctx=8192 (recommended fits under 0.8×RAM), got %d", am.ContextSize)
	}
	// 32GiB RAM >= 17GiB q4 weights → mlock on (model opts in).
	if !am.Mlock {
		t.Fatalf("cpu-only 32GiB box: want mlock=true (RAM holds weights), got false")
	}
	if am.Quant != "q4_k_m" {
		t.Fatalf("want default quant q4_k_m, got %s", am.Quant)
	}
}

// A 24 GiB CUDA box (RTX 4090 class) fully offloads Qwen3-27B q4 + KV at 8192.
func TestSynthesize_24GBFullOffload(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: []profile.GPUCard{gpuCard(24 * gib)},
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 64 * gib,
	}
	am := Synthesize(bp, modeltargets.Qwen3_27B)

	if am.NGPULayers != modeltargets.Qwen3_27B.NumLayers {
		t.Fatalf("24GiB box: want full offload nGpu=%d, got %d", modeltargets.Qwen3_27B.NumLayers, am.NGPULayers)
	}
	if am.ContextSize != 8192 {
		t.Fatalf("24GiB full offload: want ctx=8192, got %d", am.ContextSize)
	}
	if !am.Mlock {
		t.Fatalf("24GiB box with 64GiB RAM: want mlock=true, got false")
	}
	if am.NGPULayers > modeltargets.Qwen3_27B.NumLayers {
		t.Fatalf("nGpu must never exceed numLayers: got %d > %d", am.NGPULayers, modeltargets.Qwen3_27B.NumLayers)
	}
}

// A 16 GiB CUDA box (the chiribe 1M-token / 16GB-VRAM config): partial offload
// — weights+KV do not fit fully, so 0 < nGpu < numLayers and context survives.
func TestSynthesize_16GBPartial(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: []profile.GPUCard{gpuCard(16 * gib)},
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 32 * gib,
	}
	am := Synthesize(bp, modeltargets.Qwen3_27B)

	if am.NGPULayers == 0 || am.NGPULayers >= modeltargets.Qwen3_27B.NumLayers {
		t.Fatalf("16GiB box: want 0 < nGpu < %d (partial), got %d", modeltargets.Qwen3_27B.NumLayers, am.NGPULayers)
	}
	if am.ContextSize < floorContext {
		t.Fatalf("ctx below floor: got %d", am.ContextSize)
	}
	// partial offload + 32GiB RAM >= 17GiB weights → mlock on.
	if !am.Mlock {
		t.Fatalf("16GiB partial on 32GiB RAM box: want mlock=true, got false")
	}
}

// DeepSeek-V3 (671B, q4 ~374GB, 61 layers ~6GB each) on a 24GiB box: cannot
// fully offload, but does fit a few layers — honest partial offload (0 < nGpu
// < numLayers), context survives at the floor.
func TestSynthesize_DeepSeekPartial(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: []profile.GPUCard{gpuCard(24 * gib)},
		CPU: profile.CPUInfo{PhysicalCores: 16, Threads: 32},
		RAM: 64 * gib,
	}
	am := Synthesize(bp, modeltargets.DeepSeekV3)

	if am.NGPULayers == 0 {
		t.Fatalf("24GiB box fits several ~6GB DeepSeek layers: want nGpu>0, got 0")
	}
	if am.NGPULayers >= modeltargets.DeepSeekV3.NumLayers {
		t.Fatalf("24GiB box cannot fully offload 374GB model: want nGpu<%d, got %d",
			modeltargets.DeepSeekV3.NumLayers, am.NGPULayers)
	}
	if am.ContextSize < floorContext {
		t.Fatalf("ctx below floor: got %d", am.ContextSize)
	}
}

// RenderLaunchLine emits the canonical flag order and includes --mlock iff the
// matrix opts in.
func TestRenderLaunchLine_Flags(t *testing.T) {
	am := ArgMatrix{
		NGPULayers: 58, ContextSize: 8192, BatchSize: 512, Threads: 8,
		KVCacheType: "q8_0", Mlock: true, Quant: "q4_k_m", ExecBinary: "llama-server",
	}
	line := RenderLaunchLine(am, modeltargets.Qwen3_27B, "")
	want := "llama-server --model ./qwen3-27b-q4_k_m.gguf --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0 --mlock"
	if line != want {
		t.Fatalf("launch line mismatch:\nwant %q\ngot  %q", want, line)
	}

	am.Mlock = false
	lineNoMlock := RenderLaunchLine(am, modeltargets.Qwen3_27B, "/data/models/q.gguf")
	wantNoMlock := "llama-server --model /data/models/q.gguf --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0"
	if lineNoMlock != wantNoMlock {
		t.Fatalf("no-mlock launch line mismatch:\nwant %q\ngot  %q", wantNoMlock, lineNoMlock)
	}
}

// A 64 GiB Ascend 910B box fully offloads Qwen3-27B q4 and the ArgMatrix is
// CANN-aware: the backend field is cann and the fit note carries the CANN
// build advisory (the flags are standard, the server binary is not).
func TestSynthesize_CANNFullOffload(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: []profile.GPUCard{{
			Vendor: profile.VendorHuawei, Model: "910B1",
			VRAMBytes: 64 * gib, Backend: profile.BackendCANN, Index: 0,
		}},
		CPU: profile.CPUInfo{PhysicalCores: 48, Threads: 96},
		RAM: 256 * gib,
	}
	am := Synthesize(bp, modeltargets.Qwen3_27B)

	if am.NGPULayers != modeltargets.Qwen3_27B.NumLayers {
		t.Fatalf("64GiB cann box: want full offload nGpu=%d, got %d", modeltargets.Qwen3_27B.NumLayers, am.NGPULayers)
	}
	if am.Backend != profile.BackendCANN {
		t.Fatalf("want backend cann, got %s", am.Backend)
	}
	if !strings.Contains(am.FitNote, "CANN") {
		t.Fatalf("fit note must carry the CANN build advisory, got %q", am.FitNote)
	}
}

// A 16 GiB MTT S80 (musa) box gets a partial offload with the MUSA build
// advisory in the fit note.
func TestSynthesize_MUSAPartial(t *testing.T) {
	bp := profile.BoxProfile{
		GPU: []profile.GPUCard{{
			Vendor: profile.VendorMooreThreads, Model: "MTT S80",
			VRAMBytes: 16 * gib, Backend: profile.BackendMUSA, Index: 0,
		}},
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 32 * gib,
	}
	am := Synthesize(bp, modeltargets.Qwen3_27B)

	if am.NGPULayers == 0 || am.NGPULayers >= modeltargets.Qwen3_27B.NumLayers {
		t.Fatalf("16GiB musa box: want partial offload, got nGpu=%d", am.NGPULayers)
	}
	if am.Backend != profile.BackendMUSA {
		t.Fatalf("want backend musa, got %s", am.Backend)
	}
	if !strings.Contains(am.FitNote, "MUSA") {
		t.Fatalf("fit note must carry the MUSA build advisory, got %q", am.FitNote)
	}
}

// The backend advisory only fires for cann/musa: cuda and cpu boxes keep
// their initial-release fit notes byte-identical, with the backend field surfaced
// for machine consumption.
func TestSynthesize_BackendFieldWithoutAdvisory(t *testing.T) {
	cuda := profile.BoxProfile{
		GPU: []profile.GPUCard{gpuCard(24 * gib)},
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 64 * gib,
	}
	am := Synthesize(cuda, modeltargets.Qwen3_27B)
	if am.Backend != profile.BackendCUDA {
		t.Fatalf("cuda box: want backend cuda, got %s", am.Backend)
	}
	if want := "full GPU offload — weights + KV fit in VRAM"; am.FitNote != want {
		t.Fatalf("cuda fit note changed:\nwant %q\ngot  %q", want, am.FitNote)
	}

	cpu := profile.BoxProfile{
		CPU: profile.CPUInfo{PhysicalCores: 8, Threads: 16},
		RAM: 32 * gib,
	}
	am = Synthesize(cpu, modeltargets.Qwen3_27B)
	if am.Backend != profile.BackendCPU {
		t.Fatalf("cpu box: want backend cpu, got %s", am.Backend)
	}
	if want := "CPU-only — no GPU detected; context capped by 0.8×RAM, weights run from RAM"; am.FitNote != want {
		t.Fatalf("cpu fit note changed:\nwant %q\ngot  %q", want, am.FitNote)
	}
}

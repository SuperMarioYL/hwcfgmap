// Package synth implements the probe-and-emit primitive:
//
//	Synthesize(BoxProfile, ModelTarget) → ArgMatrix
//
// From the box's hardware picture + a model target, it derives the full
// llama-server argument matrix (offload layers / context / batch / threads /
// KV-cache type / mlock). This is a documented-threshold STATIC synthesis —
// it does not run tokens, does not autotune, does not exec anything. It
// collapses the per-box-per-model trial-and-error matrix to one command.
//
// All thresholds below (512 MB VRAM headroom, 0.8 RAM ceiling, physical-cores
// thread mapping, q8_0 KV footprint) are the documented knobs the synthesizer
// keys off. They are overridable per-model via profiles/<id>.yaml; the
// algorithm itself is deterministic and side-effect-free.
package synth

import (
	"math"

	"github.com/SuperMarioYL/hwcfgmap/internal/modeltargets"
	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
)

// ArgMatrix is the full llama-server argument matrix derived for one
// (box, model) pair. RenderLaunchLine turns it into the pasteable command.
type ArgMatrix struct {
	NGPULayers  int    `json:"n_gpu_layers"`  // --n-gpu-layers
	ContextSize int    `json:"context_size"`  // -c
	BatchSize   int    `json:"batch_size"`    // -b
	Threads     int    `json:"threads"`       // -t
	KVCacheType string `json:"kv_cache_type"` // --cache-type-k
	Mlock       bool   `json:"mlock"`         // --mlock
	// OffloadLayer mirrors NGPULayers — the index of the first layer kept on
	// CPU (layers [0,NGPULayers) offload to GPU). Surfaced in JSON for ops.
	OffloadLayer int    `json:"offload_layer"`
	Quant        string `json:"quant"`        // quant the matrix was sized against
	ExecBinary   string `json:"exec_binary"`  // server binary to invoke
	// FitNote is a one-line human note about the fit (e.g. "full GPU offload",
	// "partial offload — CPU bottleneck", "CPU-only, no GPU detected"). It is
	// NOT an error — it documents what the operator is getting.
	FitNote string `json:"fit_note"`
}

// Documented thresholds. Centralised so an operator can audit the synthesis.
const (
	// vramHeadroomBytes is the VRAM kept free for KV-cache, activations and
	// llama-server overhead when computing how many layers fit.
	vramHeadroomBytes = 512 * 1024 * 1024
	// ramCeiling is the fraction of host RAM the synthesizer will not exceed
	// when capping CPU-side context (leave the rest for the OS + the harness).
	ramCeiling = 0.8
	// floorContext is the minimum context the synthesizer will ever emit.
	floorContext = 2048
)

// Synthesize derives the ArgMatrix for (box, model). Pure function — no I/O,
// no exec. Behaviour:
//   - quant = the model's default quant (the operator brings the matching GGUF).
//   - offload: layers that fit in VRAM after the 512 MB headroom, accounting for
//     the per-layer KV-cache footprint at the target context. Full offload
//     when weights+KV fit; partial when only some layers fit; 0 when the box
//     has no GPU or none fit.
//   - context: the model's recommended context, capped by VRAM (offloaded KV)
//     or by 0.8×RAM (CPU-side KV), floored at 2048.
//   - threads: physical cores (llama.cpp guidance for -t).
//   - mlock: when the model opts in AND host RAM holds the full weights.
func Synthesize(bp profile.BoxProfile, mt modeltargets.ModelTarget) ArgMatrix {
	quant := mt.DefaultQuant
	qp := mt.QuantFor(quant)
	weightBytes := qp.WeightBytes
	numLayers := guardPositive(mt.NumLayers, 1)
	perLayer := safeDiv(weightBytes, uint64(numLayers))
	perLayerKV := mt.KVCachePerTokenPerLayerBytes

	ctx := guardPositive(mt.RecommendedContext, floorContext)
	threads := guardPositive(bp.CPU.PhysicalCores, 1)
	batch := guardPositive(mt.RecommendedBatch, 256)
	kvType := mt.KVCacheType
	if kvType == "" {
		kvType = "q8_0"
	}
	exec := mt.ExecBinary
	if exec == "" {
		exec = "llama-server"
	}

	vram := bp.TotalVRAM()
	ram := bp.RAM

	var nGpu int
	fitNote := ""

	switch {
	case vram == 0:
		// CPU-only box (no vendor CLI detected) — offload nothing, cap context
		// by the RAM ceiling so KV does not swap the box.
		nGpu = 0
		ctx = capContextByRAM(ctx, ram, perLayerKV, numLayers)
		fitNote = "CPU-only — no GPU detected; context capped by 0.8×RAM, weights run from RAM"
	case weightBytes <= subOrZero(vram, vramHeadroomBytes) && fitsFullOffload(weightBytes, perLayerKV, ctx, numLayers, vram):
		// weights + KV for the recommended context all fit in VRAM after headroom.
		nGpu = numLayers
		fitNote = "full GPU offload — weights + KV fit in VRAM"
	default:
		// partial: how many layers fit in (vram - headroom) when each layer
		// costs its weights + its share of KV at the target context.
		budget := subOrZero(vram, vramHeadroomBytes)
		perLayerCost := perLayer + perLayerKV*uint64(ctx)
		nGpu = int(safeDiv(budget, perLayerCost))
		if nGpu > numLayers {
			nGpu = numLayers
		}
		if nGpu < 0 {
			nGpu = 0
		}
		// If even 1 layer won't fit, the GPU is unusable for this model — be
		// honest: CPU partial with RAM-capped context.
		if nGpu == 0 {
			ctx = capContextByRAM(ctx, ram, perLayerKV, numLayers)
			fitNote = "GPU too small for any layer — CPU partial, context capped by 0.8×RAM"
		} else if nGpu < numLayers {
			// also cap context by the offloaded layers' VRAM budget so we do
			// not overpromise KV headroom on the GPU side.
			ctx = capContextByVRAM(ctx, nGpu, numLayers, perLayer, perLayerKV, vram)
			fitNote = "partial GPU offload — CPU bottleneck on the tail layers"
		}
	}

	mlock := mt.Mlock && ram >= weightBytes && weightBytes > 0

	return ArgMatrix{
		NGPULayers:   nGpu,
		ContextSize:  ctx,
		BatchSize:    batch,
		Threads:      threads,
		KVCacheType:  kvType,
		Mlock:        mlock,
		OffloadLayer: nGpu,
		Quant:        quant,
		ExecBinary:   exec,
		FitNote:      fitNote,
	}
}

// fitsFullOffload checks weights + full KV (at ctx) against (vram - headroom).
func fitsFullOffload(weightBytes, perLayerKV uint64, ctx, numLayers int, vram uint64) bool {
	kvTotal := perLayerKV * uint64(numLayers) * uint64(ctx)
	need := weightBytes + kvTotal
	budget := subOrZero(vram, vramHeadroomBytes)
	return need <= budget
}

// capContextByRAM reduces ctx so KVTotal (perLayerKV*numLayers*ctx) stays under
// ramCeiling*ram. Floors at floorContext.
func capContextByRAM(ctx int, ram, perLayerKV uint64, numLayers int) int {
	if perLayerKV == 0 || numLayers == 0 || ram == 0 {
		return clampCtx(ctx)
	}
	kvPerToken := perLayerKV * uint64(numLayers)
	maxCtx := uint64(math.Floor(float64(ram) * ramCeiling / float64(kvPerToken)))
	if maxCtx < uint64(floorContext) {
		return floorContext
	}
	if uint64(ctx) > maxCtx {
		ctx = int(maxCtx)
	}
	return clampCtx(ctx)
}

// capContextByVRAM reduces ctx so the offloaded layers' KV fits in the VRAM
// left after their weights + headroom. Only tightens; never grows ctx.
func capContextByVRAM(ctx, nGpu, numLayers int, perLayer, perLayerKV, vram uint64) int {
	if nGpu <= 0 || perLayerKV == 0 {
		return clampCtx(ctx)
	}
	budget := subOrZero(vram, vramHeadroomBytes)
	weightsGPU := perLayer * uint64(nGpu)
	kvBudget := subOrZero(budget, weightsGPU)
	// per-token KV on the offloaded layers
	kvPerToken := perLayerKV * uint64(nGpu)
	if kvPerToken == 0 {
		return clampCtx(ctx)
	}
	maxCtx := safeDiv(kvBudget, kvPerToken)
	if maxCtx < uint64(floorContext) {
		maxCtx = floorContext
	}
	if uint64(ctx) > maxCtx {
		ctx = int(maxCtx)
	}
	return clampCtx(ctx)
}

// clampCtx keeps context at the documented floor.
func clampCtx(ctx int) int {
	if ctx < floorContext {
		return floorContext
	}
	return ctx
}

func guardPositive(n, fallback int) int {
	if n <= 0 {
		return fallback
	}
	return n
}

// safeDiv is uint64 division that returns 0 instead of panicking on a 0 divisor.
func safeDiv(a, b uint64) uint64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// subOrZero is a-b clamped at 0 (no uint64 underflow).
func subOrZero(a, b uint64) uint64 {
	if b >= a {
		return 0
	}
	return a - b
}

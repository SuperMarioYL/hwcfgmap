package modeltargets

// deepseek.go holds the non-Qwen large-model in-code defaults. DeepSeek-V3 is
// the 671B MoE flagship (local runs are heavily-quantized / partial); GLM-4
// is registered at its 9B local-Chat size (the variant that fits a single
// 信创/consumer box); Kimi-K2 is the 1T MoE (local run is quantized partial).
//
// All footprints are documented-threshold estimates pending verification on
// real weights (plan schema assumptions flagged UNVERIFIED) and are
// overridable via profiles/<id>.yaml. The synthesizer is deterministic given
// these numbers; on a box whose VRAM is smaller than the q4 footprint it
// emits n-gpu-layers=0 (CPU partial) honestly rather than guessing an offload.

// DeepSeekV3 — 671B MoE, 61 layers. Real q4 footprint ~374 GB; a single box
// cannot fully offload it, so the synthesizer emits a CPU-partial line with
// capped context. That is the honest starting point for this model class.
var DeepSeekV3 = ModelTarget{
	ID:                           "deepseek-v3",
	Name:                         "DeepSeek-V3",
	Family:                       "deepseek",
	Parameters:                   671_000_000_000,
	NumLayers:                    61,
	Quants: []QuantProfile{
		{ID: "q4_k_m", WeightBytes: 374_000_000_000},
		{ID: "q8_0", WeightBytes: 715_000_000_000},
	},
	DefaultQuant:                 "q4_k_m",
	// DeepSeek-V3 uses MLA (compressed KV); per-token-per-layer footprint is
	// smaller than dense — conservative q8 estimate flagged UNVERIFIED.
	KVCachePerTokenPerLayerBytes: 1024,
	KVCacheType:                  "q8_0",
	RecommendedContext:           4096,
	RecommendedBatch:            256,
	Mlock:                        true,
	ExecBinary:                   "llama-server",
}

// GLM4 — registered at the GLM-4-9B-Chat local size: 9B params, 40 layers, q4
// footprint ~5.5 GB. This is the GLM variant that fits a 16 GB consumer/信创
// box end-to-end and is the practical local Coding Agent backbone.
var GLM4 = ModelTarget{
	ID:                           "glm-4",
	Name:                         "GLM-4 (9B Chat)",
	Family:                       "glm",
	Parameters:                   9_000_000_000,
	NumLayers:                    40,
	Quants: []QuantProfile{
		{ID: "q4_k_m", WeightBytes: 5_500_000_000},
		{ID: "q8_0", WeightBytes: 9_600_000_000},
		{ID: "f16", WeightBytes: 18_000_000_000},
	},
	DefaultQuant:                 "q4_k_m",
	KVCachePerTokenPerLayerBytes: 1536,
	KVCacheType:                  "q8_0",
	RecommendedContext:           8192,
	RecommendedBatch:            512,
	Mlock:                        true,
	ExecBinary:                   "llama-server",
}

// KimiK2 — 1T MoE (32B active). Real q4 footprint ~590 GB; like DeepSeek-V3 a
// single box cannot fully offload it; the synthesizer emits a CPU-partial
// line. Registered so `--model kimi-k2` resolves and the operator gets the
// honest "won't fit, here's the partial starting point" line.
var KimiK2 = ModelTarget{
	ID:                           "kimi-k2",
	Name:                         "Kimi-K2",
	Family:                       "kimi",
	Parameters:                   1_000_000_000_000,
	NumLayers:                    61,
	Quants: []QuantProfile{
		{ID: "q4_k_m", WeightBytes: 590_000_000_000},
		{ID: "q8_0", WeightBytes: 1_130_000_000_000},
	},
	DefaultQuant:                 "q4_k_m",
	KVCachePerTokenPerLayerBytes: 1024,
	KVCacheType:                  "q8_0",
	RecommendedContext:           4096,
	RecommendedBatch:            256,
	Mlock:                        true,
	ExecBinary:                   "llama-server",
}

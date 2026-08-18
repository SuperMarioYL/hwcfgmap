package modeltargets

// Qwen3_27B is the canonical primary model target — the m2 happy-path model
// the plan names ("在一台 16GB CUDA 盒上打印可直接粘贴的 llama-server 行 ...
// Qwen3-27B 是首个 model target"). The numbers below are documented-threshold
// estimates pending verification on real weights (the plan flags schema
// assumptions as UNVERIFIED); they drive a deterministic static synthesis and
// are overridable via profiles/qwen3-27b.yaml.
//
// Weight footprints (q4_k_m ≈ 4.6 bpw, q8_0 ≈ 8.5 bpw, f16 = 16 bpw) and the
// KV-cache per-token-per-layer figure are sized for a 27B-class dense model
// with GQA; the q8_0 KV figure is ~2 KiB/token/layer.
var Qwen3_27B = ModelTarget{
	ID:                           "qwen3-27b",
	Name:                         "Qwen3-27B",
	Family:                       "qwen3",
	Parameters:                   27_000_000_000,
	NumLayers:                    64,
	Quants: []QuantProfile{
		{ID: "q4_k_m", WeightBytes: 17_000_000_000},
		{ID: "q8_0", WeightBytes: 28_600_000_000},
		{ID: "f16", WeightBytes: 54_000_000_000},
	},
	DefaultQuant:                 "q4_k_m",
	KVCachePerTokenPerLayerBytes: 2048,
	KVCacheType:                  "q8_0",
	RecommendedContext:           8192,
	RecommendedBatch:            512,
	Mlock:                        true,
	ExecBinary:                   "llama-server",
}

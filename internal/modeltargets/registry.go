// Package modeltargets holds the per-model tuning constants the synthesizer
// keys off: parameter count, layer count, per-quant weight footprint, KV-cache
// footprint per token per layer, recommended context/batch, and the default
// KV-cache type. A ModelTarget is the "right-hand side" of
// Synthesize(BoxProfile, ModelTarget) → ArgMatrix.
//
// The registry is seeded with in-code defaults (see qwen.go / deepseek.go and
// the Default() map below) so the single binary is self-contained on a 信创 box
// with no profiles/ tree beside it. Operators can override any field by
// dropping a YAML in ./profiles/<id>.yaml (see LoadDir); the YAML is the
// canonical, human-editable source of truth for tuners.
package modeltargets

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// QuantProfile is the per-quant footprint of one model. WeightBytes is the
// total on-disk weight footprint at this quant (the synthesizer fits layers
// into VRAM against perLayerBytes = WeightBytes / NumLayers).
type QuantProfile struct {
	ID          string `yaml:"id"`            // q4_k_m, q8_0, f16, ...
	WeightBytes uint64 `yaml:"weight_bytes"` // total weight footprint at this quant
}

// ModelTarget is the full tuning contract for one model family+size.
type ModelTarget struct {
	ID   string `yaml:"id"`   // canonical id used on the CLI (--model qwen3-27b)
	Name string `yaml:"name"` // human label
	// Family is the model lineage (qwen3, deepseek, glm, kimi).
	Family string `yaml:"family"`
	// Parameters is the parameter count (e.g. 27_000_000_000 for 27B).
	Parameters uint64 `yaml:"parameters"`
	// NumLayers is the transformer layer count — drives offload layer math.
	NumLayers int `yaml:"num_layers"`
	// Quants lists the footprints at each supported quant.
	Quants []QuantProfile `yaml:"quants"`
	// DefaultQuant is the quant the synthesizer picks unless VRAM forces smaller.
	DefaultQuant string `yaml:"default_quant"`
	// KVCachePerTokenPerLayerBytes is the KV-cache footprint, in bytes, for one
	// token across one layer at the target KVCacheType. The synthesizer sizes
	// context against KVTotal = KVCachePerTokenPerLayerBytes * NumLayers * ctx.
	KVCachePerTokenPerLayerBytes uint64 `yaml:"kv_cache_per_token_per_layer_bytes"`
	// KVCacheType is the --cache-type-k value emitted (q8_0 | q6_0 | f16).
	KVCacheType string `yaml:"kv_cache_type"`
	// RecommendedContext is the -c starting point before VRAM/RAM capping.
	RecommendedContext int `yaml:"recommended_context"`
	// RecommendedBatch is the -b value (prompt batch).
	RecommendedBatch int `yaml:"recommended_batch"`
	// Mlock toggles --mlock when the box can hold the full weights in RAM.
	Mlock bool `yaml:"mlock"`
	// ExecBinary is the server binary to emit (default "llama-server").
	ExecBinary string `yaml:"exec_binary"`
}

// QuantFor returns the QuantProfile for id, or the default quant if id is
// empty / unknown. Returns the zero QuantProfile only if the target has no
// quants at all (a misconfigured target — the synthesizer guards against it).
func (m ModelTarget) QuantFor(id string) QuantProfile {
	if id != "" {
		for _, q := range m.Quants {
			if q.ID == id {
				return q
			}
		}
	}
	for _, q := range m.Quants {
		if q.ID == m.DefaultQuant {
			return q
		}
	}
	if len(m.Quants) > 0 {
		return m.Quants[0]
	}
	return QuantProfile{}
}

// Registry maps model id → ModelTarget.
type Registry struct {
	targets map[string]ModelTarget
}

// New returns a registry seeded with the in-code defaults for every supported
// model (qwen3-27b, deepseek-v3, glm-4, kimi-k2). The in-code defaults mirror
// the canonical YAMLs shipped in ./profiles so the binary works standalone.
func New() *Registry {
	r := &Registry{targets: map[string]ModelTarget{}}
	for _, mt := range []ModelTarget{Qwen3_27B, DeepSeekV3, GLM4, KimiK2} {
		r.targets[mt.ID] = mt
	}
	return r
}

// LoadDir overlays every ./profiles/*.yaml onto the in-code defaults. A YAML
// file's `id` field selects which model it overrides; missing fields fall back
// to the in-code value. A YAML with an unknown id is added as a new target.
// Missing dir is not an error — the in-code defaults stand.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // best-effort overlay; in-code defaults are authoritative
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var mt ModelTarget
		if err := yaml.Unmarshal(raw, &mt); err != nil {
			return fmt.Errorf("modeltargets: parse %s: %w", e.Name(), err)
		}
		if mt.ID == "" {
			continue
		}
		if base, ok := r.targets[mt.ID]; ok {
			r.targets[mt.ID] = mergeTarget(base, mt)
		} else {
			r.targets[mt.ID] = mt
		}
	}
	return nil
}

// Get returns the target for id and whether it exists.
func (r *Registry) Get(id string) (ModelTarget, bool) {
	mt, ok := r.targets[id]
	return mt, ok
}

// List returns the registered target ids.
func (r *Registry) List() []string {
	ids := make([]string, 0, len(r.targets))
	for id := range r.targets {
		ids = append(ids, id)
	}
	return ids
}

// mergeTarget overlays a YAML-parsed target onto an in-code base. Non-zero
// scalar fields and non-empty slices/strings from the override win; the base
// fills the gaps. This keeps a minimal override YAML valid.
func mergeTarget(base, ov ModelTarget) ModelTarget {
	out := base
	if ov.Name != "" {
		out.Name = ov.Name
	}
	if ov.Family != "" {
		out.Family = ov.Family
	}
	if ov.Parameters != 0 {
		out.Parameters = ov.Parameters
	}
	if ov.NumLayers != 0 {
		out.NumLayers = ov.NumLayers
	}
	if len(ov.Quants) > 0 {
		out.Quants = ov.Quants
	}
	if ov.DefaultQuant != "" {
		out.DefaultQuant = ov.DefaultQuant
	}
	if ov.KVCachePerTokenPerLayerBytes != 0 {
		out.KVCachePerTokenPerLayerBytes = ov.KVCachePerTokenPerLayerBytes
	}
	if ov.KVCacheType != "" {
		out.KVCacheType = ov.KVCacheType
	}
	if ov.RecommendedContext != 0 {
		out.RecommendedContext = ov.RecommendedContext
	}
	if ov.RecommendedBatch != 0 {
		out.RecommendedBatch = ov.RecommendedBatch
	}
	if ov.Mlock {
		out.Mlock = ov.Mlock
	}
	if ov.ExecBinary != "" {
		out.ExecBinary = ov.ExecBinary
	}
	return out
}

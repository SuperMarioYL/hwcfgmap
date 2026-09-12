**English** | [简体中文](README.md)

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/hero-dark.svg">
  <img src="assets/presentation/hero-light.svg" width="1000" alt="Combine a hardware profile with model-target rules to produce inspectable starting arguments for llama-server.">
</picture>

**Combine a hardware profile with model-target rules to produce inspectable starting arguments for llama-server.**

`v0.2.0` · `Go 1.24+` · [MIT](LICENSE)

[Website](https://hwcfgmap.lei6393.com) · [Demo record](docs/demo-results.json)

## Why use it

Models and memory budgets call for different offload, context and thread settings. hwcfgmap collects a BoxProfile and applies explicit weight and KV-budget rules to create an ArgMatrix. It prints settings for review without running inference or tuning throughput.

## Architecture

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/architecture-dark.svg">
  <img src="assets/presentation/architecture-light.svg" width="1000" alt="probe reads system information and available vendor CLIs. modeltargets supplies defaults and YAML overrides; synth.Synthesize derives settings from RAM, VRAM and physical cores, and RenderLaunchLine emits command text. NVMe read counters are cumulative bytes, not measured bandwidth.">
</picture>

probe reads system information and available vendor CLIs. modeltargets supplies defaults and YAML overrides; synth.Synthesize derives settings from RAM, VRAM and physical cores, and RenderLaunchLine emits command text. NVMe read counters are cumulative bytes, not measured bandwidth.

See [profile/box.go](internal/profile/box.go), [modeltargets/registry.go](internal/modeltargets/registry.go) and [synth/matrix.go](internal/synth/matrix.go). The demo calls the synthesizer with fully specified source-code input.

## Install

Requires Go 1.24+. The demo supplies an explicit hardware fixture, without probing this machine or reading model weights.

```bash
git clone https://github.com/SuperMarioYL/hwcfgmap.git
cd hwcfgmap
go build -o hwcfgmap ./cmd/hwcfgmap
```

## Quickstart

The run reads the model registry and sends a constructed eight-core, 64 GiB RAM, CPU-only profile through the real synthesizer. The output is a static estimate, not a tested or optimal configuration.

```bash
go run ./cmd/hwcfgmap models
go run ./examples/presentation
```

The complete input is [examples/presentation/main.go](examples/presentation/main.go). ./example.gguf in the emitted line is a path to replace; no file is opened or executed.

## Usage

probe emits BoxProfile. probe --model ID adds arguments and a launch line; --launch-only emits just the line, and --gguf sets its model path. models lists targets and --profiles selects a YAML directory. Hardware-probe completeness depends on system facilities and vendor CLIs.

## Recorded demo

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/process-dark.svg">
  <img src="assets/presentation/process-light.svg" width="1000" alt="The run reads the model registry and sends a constructed eight-core, 64 GiB RAM, CPU-only profile through the real synthesizer. The output is a static estimate, not a tested or optimal configuration.">
</picture>

### Read model rules

List the local targets with default context and quant.

```text
$ go run ./cmd/hwcfgmap models
qwen3-27b       Qwen3-27B               q=q4_k_m  ctx=8192  layers=64
deepseek-v3     DeepSeek-V3             q=q4_k_m  ctx=4096  layers=61
glm-4           GLM-4 (9B Chat)         q=q4_k_m  ctx=8192  layers=40
kimi-k2         Kimi-K2                 q=q4_k_m  ctx=4096  layers=61
```

### Synthesize fixture settings

Generate arguments and an unexecuted launch line from the complete CPU-only input.

```text
$ go run ./examples/presentation
{
  "input": {
    "gpu": [],
    "cpu": {
      "physical_cores": 8,
      "threads": 16,
      "numa_nodes": 0,
      "model": "fixture CPU"
    },
    "ram_bytes": 68719476736
  },
  "arguments": {
    "n_gpu_layers": 0,
    "context_size": 8192,
    "batch_size": 512,
    "threads": 8,
    "kv_cache_type": "q8_0",
    "mlock": true,
    "offload_layer": 0,
    "quant": "q4_k_m",
    "exec_binary": "llama-server",
    "backend": "cpu",
    "fit_note": "CPU-only — no GPU detected; context capped by 0.8×RAM, weights run from RAM"
  },
  "launch": "llama-server --model ./example.gguf --n-gpu-layers 0 -c 8192 -b 512 -t 8 --cache-type-k q8_0 --mlock"
}
```

## Capabilities and integration

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/integrations-dark.svg">
  <img src="assets/presentation/integrations-light.svg" width="1000" alt="Built-in target rules include qwen3-27b, glm-4, deepseek-v3 and kimi-k2. Check the YAML against the actual weights and runtime version, then validate startup and throughput on the target hardware.">
</picture>

Built-in target rules include qwen3-27b, glm-4, deepseek-v3 and kimi-k2. Check the YAML against the actual weights and runtime version, then validate startup and throughput on the target hardware.



## Configuration

YAML fields include num_layers, quants[].weight_bytes, default_quant, kv_cache_per_token_per_layer_bytes, kv_cache_type, recommended_context, recommended_batch, mlock and exec_binary. Static rules reserve VRAM headroom and cap context using RAM; they are not a full topology-aware memory simulation.

## Roadmap and scope

Hardware profiles, available vendor probes, the registry and static synthesis are implemented. v0.2.0 lands domestic-card VRAM parsing: the Ascend `npu-smi info` table (NPU model + HBM/Memory-Usage pools, parsing pinned against captured 910B1 / 910PremiumA output) and Moore Threads `mthreads-gmi -q -j` JSON (product name + memory_total). Unrecognized output shapes always degrade to the honest zero-VRAM presence probe — never a guessed number. Synthesis output carries a backend field and a build advisory on CANN/MUSA boxes. Biren bre-smi remains presence-only (no verifiable public output format). Validation on physical hardware, fleet synchronization and certified profiles remain future work; no live paid fleet product is established here.

- Settings come from static rules, not measured optima or deployment guarantees.
- Domestic-card parsing is verified against captured CLI output shapes, not on physical hardware; this example does not validate GPU backends or multi-device topology.
- NVMe byte counters are not throughput rates.

[Terminal recording](assets/demo.gif) · [Recording script](docs/demo.tape)

## License

[MIT](LICENSE)

# Changelog

## [0.2.0] - 2026-09-12

- CLI contracts: `probe -h` / `models -h` now exit 0 with clean usage output (previously they printed `flag: help requested` and exited 1), and unexpected positional arguments are rejected with a did-you-mean error instead of being silently ignored — `hwcfgmap probe qwen3-27b` no longer prints probe-only output.
- Launch line: `--gguf` paths containing whitespace or shell metacharacters are now POSIX-quoted, so the pasteable `llama-server` line always carries exactly one `--model` argument; clean paths render byte-identically to before.
- Domestic-card probing (the m3 roadmap item): Ascend `npu-smi info` tables are parsed for real per-NPU VRAM (largest Memory-Usage/HBM-Usage pool, pinned against captured 910B1 / 910PremiumA output), and Moore Threads `mthreads-gmi -q -j` JSON is parsed for product name + memory total. Unrecognized output shapes degrade to the documented zero-VRAM presence stub — never a guessed number. Biren `bre-smi` remains presence-only.
- CANN/MUSA-aware synthesis: `ArgMatrix` gains a `backend` field and the fit note carries a build advisory (CANN / MUSA llama-server build) when a domestic card gets GPU offload.
- Version lockstep: every live surface (VERSION, CLI version constant, README badges, site meta) now reports 0.2.0; frozen initial-release recordings (`docs/demo-results.json`, `assets/demo.gif`) are untouched.

## [0.1.0] - 2026-08-18

- Initial release: hardware probe (GPU vendor CLIs incl. the domestic-card framework, CPU, RAM, NVMe), model-target registry with YAML overrides (Qwen3-27B, DeepSeek-V3, GLM-4, Kimi-K2), static llama-server argument synthesis (offload / context / batch / threads / KV-cache / mlock), and the `probe` / `models` / `version` CLI.

#!/usr/bin/env bash
# probe-and-emit in three lines: probe the box, pick a model, paste the line.
set -euo pipefail

# 1. probe the box and synthesize a model target → one pasteable llama-server line
LINE=$(hwcfgmap probe --model qwen3-27b --launch-only)
echo "$LINE"

# 2. (operator) point --model at the real GGUF on disk
# hwcfgmap probe --model qwen3-27b --gguf /data/models/qwen3-27b-q4_k_m.gguf --launch-only

# 3. (operator) paste the line into a terminal — the model loads with box-tuned args

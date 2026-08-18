<div align="right"><sub><b>简体中文</b>&nbsp;&nbsp;⇄&nbsp;&nbsp;<a href="./README.en.md">English</a></sub></div>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./assets/hero-light.svg">
    <img src="./assets/hero-light.svg" width="880" alt="hwcfgmap — probe the box, emit the llama-server line">
  </picture>
</p>

<p align="center"><sub>信创与本地 GPU 盒子的硬件探测工具，一键合成 llama.cpp 最优服务启动参数。</sub></p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-0071E3" alt="license"></a>
  <a href="https://github.com/SuperMarioYL/hwcfgmap/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/hwcfgmap?label=release&color=0071E3" alt="release"></a>
  <a href="https://github.com/SuperMarioYL/hwcfgmap/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/hwcfgmap/ci.yml?label=CI&color=10A37F" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white" alt="go">
  <img src="https://img.shields.io/badge/Coding%20Agent-ready-5E5CE6" alt="Coding Agent">
  <img src="https://img.shields.io/badge/Agent-ready-10A37F" alt="Agent">
</p>

**停止逐卡逐模型手调 llama-server 参数——一条命令探测信创/本地盒子，直接合成可粘贴的 `llama-server` 启动行。** hwcfgmap 读 GPU（含国产卡 vendor CLI `npu-smi`/`mthreads-smi`）、CPU、RAM、NVMe，按本机硬件 + 目标国产模型（Qwen3 / DeepSeek / GLM-4 / Kimi-K2）合成 `--n-gpu-layers` / `-c` / `-b` / `--cache-type-k` / `-t` / `--mlock`，从探测到拿到启动行 < 10 秒。

## 为什么是现在

llama.cpp 把 `llama-server` 的旋钮全暴露了，但**合成一个都不做**——每换一个模型、每换一张卡，运维就得刷模型页、跑试错、手改启动行。当 27B 级模型跨过 16GB 显存的消费线，逐盒调参从"一次设置"变成"每周任务"。而在信创 / 数据不出境的国产 GPU 盒（昇腾 910B / 摩尔线程 MTT S80 / 壁仞 BR100）上，llama.cpp 的 offload/quant/KV-cache 矩阵**公开文档为零**——手调不是烦，是无图可循。这正是 [headroomlabs-ai/headroom](https://github.com/headroomlabs-ai/headroom) 这类 Coding Agent 和 [affaan-m/ECC](https://github.com/affaan-m/ECC) 这类 Agent 直接坐在 `llama-server` 参数层、把调参负担继承给运维的缺口——它们需要一条 Agent 可直接消费的、按本机调好的启动行。hwcfgmap 补上这道缝：探测盒子，直接合成参数，让本地 Agent 不再逐卡逐模型试错。

## 目录

- [架构](#架构)
- [安装与快速开始](#安装与快速开始)
- [用法](#用法)
- [Demo](#demo)
- [配置](#配置)
- [路线图](#路线图)
- [付费](#付费)
- [许可证](#许可证)

## <img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 架构

单二进制、单进程、无 daemon。仅在探测阶段 exec 盒子上已有的 vendor CLI（`nvidia-smi` / `rocm-smi` / `npu-smi` / `mthreads-smi`），不引入第二个常驻进程。核心原语是 **probe-and-emit** 合成函数 `Synthesize(BoxProfile, ModelTarget) → ArgMatrix`：从硬件画像 + 模型目标，直接合成 llama.cpp 全参数矩阵。

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/atlas-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/atlas-light.svg">
  <img src="./assets/atlas-light.svg" width="880" alt="架构：信创/GPU 盒子 → probe 探测 → BoxProfile → synth(+ModelTarget) → ArgMatrix → llama-server 启动行">
</picture>

核心数据流：`probe` 读 `/proc` + vendor CLI → `BoxProfile`（GPU/CPU/RAM/NVMe，含信创卡 backend 映射）→ `synth` 用 `BoxProfile` + `ModelTarget`（每模型的 VRAM/KV 预算，YAML 可覆盖）做**静态合成** → `ArgMatrix`（offload 层数 / context / batch / threads / KV-cache type / mlock）→ `RenderLaunchLine` 拼成可直接粘贴的 `llama-server` 行。**不跑 token、不 exec llama-server**——只打印命令行，运维自己粘贴。

## <img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 安装与快速开始

```bash
go install github.com/SuperMarioYL/hwcfgmap/cmd/hwcfgmap@latest   # 1. 安装（<30s）
hwcfgmap probe --model qwen3-27b                                   # 2. 探测盒子 + 合成参数（<10s）
# 3. 复制打印出的 llama-server ... 行，粘贴进终端回车
```

> 信创盒无 Go 工具链时，从 [releases](https://github.com/SuperMarioYL/hwcfgmap/releases) 下载 Linux 二进制即可——单文件，无运行时依赖。

<details><summary>示例输出（无 GPU 的开发机）</summary>

```
hwcfgmap: probing box (GPU / CPU / RAM / NVMe)...
{
  "gpu": [],
  "cpu": { "physical_cores": 10, "threads": 10, "model": "Apple M1 Max" },
  "ram_bytes": 68719476736
}

# ArgMatrix
{
  "n_gpu_layers": 0,
  "context_size": 8192,
  "batch_size": 512,
  "threads": 10,
  "kv_cache_type": "q8_0",
  "mlock": true,
  "quant": "q4_k_m",
  "fit_note": "CPU-only — no GPU detected; context capped by 0.8×RAM, weights run from RAM"
}

# paste into your terminal (edit --model to your GGUF path):
llama-server --model ./qwen3-27b-q4_k_m.gguf --n-gpu-layers 0 -c 8192 -b 512 -t 10 --cache-type-k q8_0 --mlock
```

</details>

没有 GPU 时，hwcfgmap 诚实输出一条 CPU-only 的 `llama-server` 行（`--n-gpu-layers 0`，context 按 0.8×RAM 封顶，能装下权重时 `--mlock`）。这是起点不是最优解——见 [配置](#配置) 与 [路线图](#路线图)。

## <img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 用法

```bash
# 只探测盒子，打印 BoxProfile JSON（m1 核心）
hwcfgmap probe

# 探测 + 合成指定模型，打印 BoxProfile + ArgMatrix + 启动行
hwcfgmap probe --model qwen3-27b
hwcfgmap probe --model glm-4
hwcfgmap probe --model deepseek-v3

# 只打印启动行（供脚本捕获）
hwcfgmap probe --model qwen3-27b --launch-only

# 把你的 GGUF 路径直接烤进启动行
hwcfgmap probe --model qwen3-27b --gguf /data/models/qwen3-27b-q4_k_m.gguf --launch-only

# 列出已注册的模型目标
hwcfgmap models
```

模型目标来自代码内默认值 + `./profiles/*.yaml` 覆盖（见 [配置](#配置)）。合成是**确定性静态合成**：基于已文档化的阈值（512MB 显存余量、0.8×RAM 上限、物理核数→`-t`、q8_0 KV 占用），不跑 token、不 autotune、不 exec——合成结果可复现。

## <img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Demo

![demo](assets/demo.gif)

`hwcfgmap probe --model qwen3-27b`：探测盒子 → 打印 BoxProfile JSON → 合成 ArgMatrix → 输出可粘贴的 `llama-server` 行。完整脚本见 [`docs/demo.tape`](./docs/demo.tape)，CI 在 [`demo.yml`](./.github/workflows/demo.yml) 用 vhs 重新渲染。

## <img src="https://api.iconify.design/tabler:adjustments.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 配置

模型目标是 `./profiles/<id>.yaml`，覆盖代码内默认值（缺省字段回退到内置值，见 [`internal/modeltargets/registry.go`](./internal/modeltargets/registry.go)）。已注册目标：

| 模型 id | 名称 | 默认 quant | 推荐 context | 层数 |
|---|---|---|---:|---:|
| `qwen3-27b` | Qwen3-27B | q4_k_m | 8192 | 64 |
| `glm-4` | GLM-4 (9B Chat) | q4_k_m | 8192 | 40 |
| `deepseek-v3` | DeepSeek-V3 | q4_k_m | 4096 | 61 |
| `kimi-k2` | Kimi-K2 | q4_k_m | 4096 | 61 |

`profiles/qwen3-27b.yaml` 顶层字段：

| 字段 | 类型 | 默认 | 含义 |
|---|---|---|---|
| `id` | string | `qwen3-27b` | CLI `--model` 用的目标 id |
| `num_layers` | int | 64 | 层数，驱动 offload 计算 |
| `quants[].weight_bytes` | uint64 | 见 YAML | 每个 quant 的权重总占用（bytes） |
| `default_quant` | string | `q4_k_m` | 合成所依据的 quant（运维自带对应 GGUF） |
| `kv_cache_per_token_per_layer_bytes` | uint64 | 2048 | 单 token 单层 KV 占用，驱动 context 封顶 |
| `kv_cache_type` | string | `q8_0` | `--cache-type-k` 的值 |
| `recommended_context` | int | 8192 | `-c` 起点，再被 VRAM/RAM 封顶 |
| `recommended_batch` | int | 512 | `-b` |
| `mlock` | bool | true | RAM 装得下权重时是否 `--mlock` |

## <img src="https://api.iconify.design/tabler:map-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 路线图

- [x] **m1 探测硬件**：`hwcfgmap probe` 读取 GPU/CPU/RAM/NVMe（含国产卡 vendor CLI 框架，先 stub），打印 `BoxProfile` JSON；CUDA + ROCm 探测走通
- [x] **合成 + CLI**：`Synthesize(BoxProfile, ModelTarget) → ArgMatrix` + `RenderLaunchLine`，`probe --model` 直接打印可粘贴的 `llama-server` 行
- [ ] **m3 国产卡落地**：昇腾 910B / 摩尔线程 MTT S80 vendor CLI 探测落地，产出 CANN/MUSA-aware 启动行；补 DeepSeek-V3 / GLM-4 / Kimi-K2 的真实硬件验证
- [ ] **v0.2 fleet sync**：多盒配置同步 + 厂商认证的昇腾/摩尔线程 profile 包（商业化）

### hwcfgmap vs 手调（llama-bench + HF 模型卡）

| 维度 | hwcfgmap | 手调（llama-bench + HF 卡） |
|---|:---:|:---:|
| 探测本机硬件自动合成全参数矩阵 | ✓ | ✗（每 combo 给吞吐数，不给该用哪个 combo） |
| 按 VRAM/cores 封顶 context/offload | ✓ | partial（靠手算） |
| 信创卡（昇腾/摩尔线程/壁仞）backend 映射 | ✓ | —（公开文档为零） |
| 一条可粘贴的 `llama-server` 行 | ✓ | ✗（旋钮散在文档里） |
| 跑 token 验证吞吐 | —（只合成，不验证） | ✓ |

hwcfgmap 合成不验证——它给你按本机调好的**起点**，不是跑过 token 的最优解。要测吞吐仍用 `llama-bench`；hwcfgmap 替代的是"该用哪个 combo"的试错，不是吞吐基准。

## <img src="https://api.iconify.design/tabler:cash-banknote.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 付费

hwcfgmap 的 OSS 核心（探测 / 合成 / CLI + 代码内模型默认值）**永久免费**，MIT 协议——信创运维单盒就能跑通。商业化路径明确且**绝不削弱 OSS 核心**：付费 Team 档提供多盒配置同步 + 厂商认证的昇腾/摩尔线程 profile 包，覆盖 llama.cpp 上游结构性不做、国产卡厂商也无配置文档的硬地盘。

- **Team 档**：¥19,800/年/团队（≤5 盒），超出盒数 ¥3,980/盒/年。含 fleet 配置同步、厂商认证 profile 包、离线 Ed25519 license（数据不出境，无 phone-home）。
- **企业档**：air-gapped 信创部署的离线企业许可 + 增值税专用发票，按对公合同结算。

OSS CLI 永远免费；付费的是结构性无文档的国产卡 profile 覆盖 + fleet 同步。Team 档推迟到 v0.2。首条付费路径：运维在盒 1 跑通免费 `hwcfgmap probe` → 命中"再加 3 台昇腾、profile 漂移、无同步"→ 升级 Team → 1 小时 fleet 同步 demo → 对公合同 + 专票。

## <img src="https://api.iconify.design/tabler:license.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 许可证

[MIT](./LICENSE) © 2026 SuperMarioYL。提 issue 或 PR 见 [issues](https://github.com/SuperMarioYL/hwcfgmap/issues)。

## 分享

```
hwcfgmap — 信创/本地 GPU 盒一条命令出 llama-server 启动行，告别逐卡逐模型手调。本地 Agent 直接消费按本机调好的 offload/context/KV-cache。https://github.com/SuperMarioYL/hwcfgmap
```

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>

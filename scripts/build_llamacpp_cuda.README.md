# build_llamacpp_cuda.sh — 跨机器复现手册

把 [`build_llamacpp_cuda.sh`](./build_llamacpp_cuda.sh) 在 **任意 WSL2 / Linux** 上跑出一份带 CUDA 加速的 `llama.cpp`(包含 `llama-cli` / `llama-server` / `llama-bench`)。本文记录了在不同硬件 / 发行版上的**实测可用命令组合**。

## 0. 前置条件(WSL2)

1. Windows 主机已装 **NVIDIA Game Ready / Studio driver ≥ 535** —— 这一步装的就是 WSL2 GPU 直通用的 driver,**WSL 内不需要再装 driver**。
2. `wsl --shutdown` 后重启 WSL,确保 `/usr/lib/wsl/lib/libcuda.so.1` 已经存在(可用 `ls /usr/lib/wsl/lib/libcuda*` 验证)。
3. **`/etc/wsl.conf` 里 `interop` / `appendWindowsPath` 是否 false 都不影响 GPU 直通**。NVIDIA WSL2 GPU 走的是独立机制,不依赖 interop。本脚本完全在 Linux 侧完成,不需要调用任何 Windows 可执行文件。
4. 留出磁盘:CUDA Toolkit 13.x 约 **3.5 GB**,llama.cpp 源码 + build 约 **1.5 GB**,合计 **≥ 5 GB** 富余。

## 1. 推荐操作流程

### 1.1 通用一键流程(交互模式)

```bash
git clone <项目> && cd <项目>
bash scripts/build_llamacpp_cuda.sh
# 中途会两次 sudo 提权(装 cuda-toolkit / 装 cmake),并在装 toolkit 前 confirm 一次
```

跑完后:

- 二进制: `~/src/llama.cpp/build/bin/{llama-cli,llama-server,llama-bench}`
- `~/.bashrc` 已写入 `/usr/local/cuda/bin` 与 `LD_LIBRARY_PATH`,新开终端 `nvcc --version` 可用
- 终端最后会打印一段"VRAM 建议",告诉你这台卡能跑什么规格的模型

### 1.2 不交互(适合 SSH / 长任务,但需先有 sudo NOPASSWD 或交互过一次)

```bash
ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh
```

⚠ 如果 sudo 需要密码,先在前台跑一次到 cuda 阶段把密码缓存住:

```bash
ASSUME_YES=1 STAGE=cuda bash scripts/build_llamacpp_cuda.sh   # 前台输入密码
ASSUME_YES=1 STAGE=deps bash scripts/build_llamacpp_cuda.sh   # 前台再输一次
ASSUME_YES=1 STAGE=fetch bash scripts/build_llamacpp_cuda.sh
nohup bash -c 'ASSUME_YES=1 STAGE=build bash scripts/build_llamacpp_cuda.sh \
  && ASSUME_YES=1 STAGE=verify bash scripts/build_llamacpp_cuda.sh' \
  > /tmp/llama_cuda_build.log 2>&1 &
disown
```

### 1.3 分阶段(WSL 容易抖动时优选)

`STAGE=detect|cuda|deps|fetch|build|verify`,详见脚本头注释。

## 2. 自动决策矩阵

脚本会自动探测并选择:

| 维度 | 自动来源 | 默认行为 | 何时需要手动覆盖 |
|---|---|---|---|
| `NVIDIA_DISTRO` | `/etc/os-release` | debian13 / debian12 / ubuntu24.04 / 22.04 / 20.04 自动映射 | 你的 distro 不在 NVIDIA 官方支持列表里 |
| `CUDA_ARCHS` | `nvidia-smi --query-gpu=compute_cap` | 多 GPU 自动取并集 | WSL 内还没装 toolkit 因此 `nvidia-smi` 缺席 |
| `CUDA_PKG` | `recommend_cuda_pkg()` | sm_100/120 (Blackwell) → `cuda-toolkit-13-2`;其它 → `cuda-toolkit-12-6`;driver < 580 自动从 13.x 降到 12.6 | 你需要某个特定 CUDA 版本(比如对齐 PyTorch 12.4) |
| `LLAMA_SRC` / `LLAMA_REF` | 默认 `~/src/llama.cpp` / `master` | — | 想固定到某个 release tag 或换源码目录 |

`detect_env` 末尾会打印一段"决策汇总表",上面写明每个值的来源(`auto` / `env` / `nvidia-smi`),不用再翻代码也不用试运行猜行为。

## 2.1 各 GPU 的"已知好"命令组合(参考)

下表第二列是**已装 `nvidia-smi`** 时实际需要传的变量(其它都让脚本自动)。

| GPU(架构, VRAM) | 完整命令(假设主机 distro 已被自动识别) |
|---|---|
| **RTX 5060/5070/5080/5090**(Blackwell, sm_120) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| **RTX 4060/4070/4080/4090, L4**(Ada, sm_89) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| **RTX 3050 Laptop / RTX 30xx, A4000-A6000**(Ampere, sm_86) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| **A100**(Ampere, sm_80) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| **RTX 20xx, T4**(Turing, sm_75) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| **H100 / H200**(Hopper, sm_90) | `ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh` |
| 多 GPU 异构(如 86 + 89) | 同上,脚本自动取并集 → `CUDA_ARCHS=86;89` |
| **WSL 内还没 nvidia-smi**(toolkit 安装前) | 必须显式提示 GPU,例如 `CUDA_ARCHS=86 bash ...` |
| **想强行用 CUDA 12.x**(对齐 PyTorch / 老 driver) | `CUDA_PKG=cuda-toolkit-12-6 bash ...` |
| **distro 探测失败 / 用旧仓库** | `NVIDIA_DISTRO=ubuntu2204 bash ...` |

> 不在 NVIDIA 仓库里的 distro(老 Debian, Arch, Fedora 等)需要换装方式;本脚本只覆盖 Debian/Ubuntu 的 apt 路径。

## 3. 目标机器示例:RTX 3050 Laptop 4GB / Debian 13 WSL / 已装 nvidia-smi / interop=false

```bash
# 0) 前置(WSL 内验证一下;不够才回 Windows 端调)
ls /usr/lib/wsl/lib/libcuda*       # 看到 libcuda.so.1 → GPU 直通 OK
nvidia-smi                         # 看到 RTX 3050 + driver 版本

# 1) clone 项目并切到目录
git clone <project_repo> ~/work && cd ~/work

# 2) 先单跑 detect 阶段确认决策无误(不会动系统)
STAGE=detect bash scripts/build_llamacpp_cuda.sh
# 应该看到决策汇总:
#   GPU            : NVIDIA GeForce RTX 3050 ... Laptop GPU
#   driver_version : 5xx.xx
#   VRAM           : 4096 MiB  → ≥4GB:  4B Q4_K_M 全 offload / 7B Q4 ngl ~18~22 (混合) / 不建议 9B+
#   distro         : debian13
#   CUDA_ARCHS     : 86                    (来源: nvidia-smi)   ← 自动识别
#   CUDA_PKG       : cuda-toolkit-12-6     (来源: auto)         ← 自动选择(不是 Blackwell → 12.6)

# 3) 真跑(全部 auto,不用任何额外变量)
ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh
# 中途:
#   - 装 cuda-toolkit-12-6 约 2.5 GB(比 13-2 小)
#   - 装 cmake/ninja/git/libcurl/libssl
#   - clone llama.cpp + ninja 编译 5~30 分钟(取决于 CPU 核数)

# 4) 让新二进制可用
echo 'export PATH=$HOME/src/llama.cpp/build/bin:$PATH' >> ~/.bashrc
source ~/.bashrc
which llama-cli                    # 应指向 ~/src/llama.cpp/build/bin/llama-cli
llama-cli --list-devices           # 应看到 RTX 3050 / Ampere

# 5) 拉一个 4 GB 卡能跑的模型(示例: Qwen2.5-3B-Instruct Q4_K_M ≈ 1.9 GB)
llama-cli -hf Qwen/Qwen2.5-3B-Instruct-GGUF:Q4_K_M --no-webui

# 6) 跑基准
llama-bench \
  -m ~/.cache/huggingface/hub/models--Qwen--Qwen2.5-3B-Instruct-GGUF/snapshots/*/qwen2.5-3b-instruct-q4_k_m.gguf \
  -ngl 99 -p 256 -n 128 -r 3
# RTX 3050 4GB 上预期: pp256 ≈ 1500~2000 t/s, tg128 ≈ 60~80 t/s
```

> **关键变化**:从前需要 `NVIDIA_DISTRO=ubuntu2204 CUDA_ARCHS=86`,现在脚本通过 `nvidia-smi` 自动拿到 sm_86,通过 `/etc/os-release` 识别到 debian13,通过 `recommend_cuda_pkg()` 自动选 `cuda-toolkit-12-6`(因为不是 Blackwell)——一行命令完成。

## 3.1 没装 nvidia-smi 的边角场景

部分 WSL 镜像装了 driver 但没拉 toolkit,WSL 内 `command -v nvidia-smi` 会失败。脚本会自动改用 `/usr/lib/wsl/lib/libcuda.so.1` 的存在性确认 GPU 直通,但**无法**自动拿到 `compute_cap`,此时必须显式给:

```bash
# RTX 3050 Laptop(Ampere)
ASSUME_YES=1 CUDA_ARCHS=86 bash scripts/build_llamacpp_cuda.sh
```

装完 toolkit 后再跑就完全 auto 了(因为 `/usr/local/cuda/bin/nvidia-smi` 出现了)。

## 4. 容易踩的坑(按出现频率排序)

| 现象 | 根因 | 处理 |
|---|---|---|
| `apt-get update` 报 `Sub-process /usr/bin/sqv returned an error code (1)` | Debian 13 的 `sqv` 拒 NVIDIA `ubuntu2204` 仓库 SHA1 keyring,但 `cuda-keyring 1.1-1` 会同时塞 ubuntu2204 + debian13 两份 source | 脚本 `keep_only_active_distro_source()` 自动禁用不匹配的 source(改为 `*.disabled`)。如果你后续手动 `apt-get install cuda-*` 还是报错,把 `/etc/apt/sources.list.d/cuda-ubuntu2204-x86_64.list.disabled` 留着别恢复 |
| `nvcc: command not found`(装完后) | 新 shell 没 source `~/.bashrc` | `source ~/.bashrc` 或开新终端 |
| `--list-devices` 看不到 CUDA | `CUDA_ARCHS` 设错(比如把 86 当成 120) | 重置 `rm -rf $LLAMA_SRC/build` 再 `STAGE=build CUDA_ARCHS=<正确值>` 重编 |
| WSL 内 `nvidia-smi` 找不到,但 Windows 端能看到 GPU | 没装 cuda-toolkit-* 之前 WSL 里通常没 nvidia-smi,只有 `/usr/lib/wsl/lib/libcuda.so.1` | 这是正常的;脚本会先用 libcuda.so.1 判断,要求你显式 `CUDA_ARCHS=...`。装完 toolkit 后 `/usr/local/cuda/bin/nvidia-smi` 就出现了 |
| `cuda-toolkit-13-2` 装不上(404) | distro tag 错了 | 用 `NVIDIA_DISTRO=ubuntu2204`(或当前发行版对应值),并核对 https://developer.download.nvidia.com/compute/cuda/repos/ |
| `sudo: a terminal is required` | 后台脚本碰上 sudo 密码 | 见 §1.2,先在前台缓存密码 |
| 编译卡在 `nvcc fatal: Unsupported gpu architecture 'compute_120'` | 用了 CUDA ≤ 12.x 但指了 `CUDA_ARCHS=120`(Blackwell 需 CUDA 13.x) | 升 `CUDA_PKG=cuda-toolkit-13-2`,或把 `CUDA_ARCHS` 降到目标 GPU 的真实值 |
| 编译完了但运行 `Illegal instruction` | `-DGGML_NATIVE=ON` 在编译机和运行机不同(主要是 AVX/AVX2 不一致)。同机编同机用一般不会触发 | 加 `cmake ... -DGGML_NATIVE=OFF` 重编(脚本里需要手改) |

## 5. 卸载 / 清理

```bash
# 卸 CUDA toolkit (释放 ~3.5 GB)
sudo apt-get remove --purge -y 'cuda-toolkit-*' 'cuda-cccl-*' 'cuda-nvcc-*'
sudo apt-get autoremove -y

# 删源码 + build (释放 ~1.5 GB)
rm -rf ~/src/llama.cpp

# 撤回 ~/.bashrc 的 PATH 改动(如果加过)
sed -i '/Added by build_llamacpp_cuda.sh/,+2 d' ~/.bashrc
sed -i '\|llama.cpp/build/bin|d' ~/.bashrc
```

## 6. 与 Nobody 工程对接

> **Disclaimer**：本脚本是 GPU 加速参考实现。Nobody 的默认 `nobody.yaml` 假设 operator 已经用 `brew install llama.cpp` / `apt install llama.cpp` 装好了 CPU 版 llama.cpp（最低门槛、零硬件依赖）。仅当 (a) 你想榨干本地 NVIDIA GPU、且 (b) 愿意接受脚本里的 CUDA Toolkit 安装（≈3.5 GB），才需要走这一脚本。
>
> CPU 默认路径：直接 `make llamacpp-up`（启 `qwen3.5-4b`，纯 CPU、ctx=8192）。
> GPU 加速路径（本脚本编出来后）：`LLAMA_NGL=99 LLAMA_CHAT_MODEL=$HOME/models/qwen3.5-9b-q4_k_m.gguf make llamacpp-up`。
> 想自动判断当前机器最佳配置：`scripts/recommend_llamacpp_yaml.sh`。

编出来的 `llama-server` 直接对接 `internal/inference/llamacpp/` 走 OpenAI compat。配置项见 [`docs/framework/llamacpp-cheatsheet.md`](../docs/framework/llamacpp-cheatsheet.md)；性能基准、调参经验见 [`docs/benchmarks/qwen35-9b-llamacpp-vs-ollama.md`](../docs/benchmarks/qwen35-9b-llamacpp-vs-ollama.md)。

#!/usr/bin/env bash
# 在 WSL2 / Linux 上从源码编译启用 CUDA 的 llama.cpp（含 llama-server / llama-cli / llama-bench）。
#
# 通用性 / 跨机器复现（一切都能 auto，但每个决策都允许 export 显式覆盖）：
# - 自动探测 distro (debian13 / debian12 / ubuntu2404 / ubuntu2204 / ubuntu2004)；与本机不一致会 warn。
# - 自动探测 GPU compute capability (Turing/Ampere/Ada/Hopper/Blackwell)；
#   多卡自动取并集；探测失败拒绝硬编码、强制要求 CUDA_ARCHS。
# - 自动选 CUDA Toolkit 包 (cuda-toolkit-13-2 vs 12-6)：Blackwell 强制 13.x；
#   其余看 driver 版本（< 580 自动降到 12.6）。
# - 自动检测已装组件（cuda-keyring / nvcc / cmake / ninja / 源码 / 编译产物），跳过冗余步骤。
# - 兼容 WSL2 GPU 直通：未装 toolkit 时 nvidia-smi 可能不存在，但只要
#   /usr/lib/wsl/lib/libcuda.so.1 存在就视为直通正常（与 /etc/wsl.conf 的 interop 开关无关）。
# - detect_env 末尾打印一份"决策汇总表"，告诉你接下来会装什么、为什么、怎么覆盖。
# - 自动按显存给出可跑模型建议（4GB / 8GB / 16GB / 24GB）。
#
# 设计原则：
# - 幂等：可重复执行；已装的 apt 包跳过；已 clone 的仓库执行 fetch+checkout。
# - 显式：所有"装系统包 / 改 PATH"动作前会询问确认，可用 ASSUME_YES=1 跳过提问。
# - 可分步：用 STAGE 环境变量控制只跑某一阶段，便于调试和 WSL 抖动恢复。
#
# 用法：
#   bash scripts/build_llamacpp_cuda.sh                # 全流程（交互）
#   ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh   # 不交互（CI / WSL 长任务）
#   STAGE=build bash scripts/build_llamacpp_cuda.sh    # 只跑指定阶段
#
# 跨机器示例：
#   # 1) Debian 13 + RTX 5060 (Blackwell, 8GB)：本机默认值即可
#   ASSUME_YES=1 bash scripts/build_llamacpp_cuda.sh
#
#   # 2) Ubuntu 22.04 WSL + RTX 3050 Laptop (Ampere, 4GB)：
#   ASSUME_YES=1 NVIDIA_DISTRO=ubuntu2204 CUDA_ARCHS=86 \
#     bash scripts/build_llamacpp_cuda.sh
#
#   # 3) Ubuntu 24.04 WSL + RTX 4090 (Ada, 24GB)：
#   ASSUME_YES=1 NVIDIA_DISTRO=ubuntu2404 CUDA_ARCHS=89 \
#     bash scripts/build_llamacpp_cuda.sh
#
# 关键环境变量：
#   LLAMA_SRC          源码目录 (默认 $HOME/src/llama.cpp)
#   LLAMA_REF          要 checkout 的 ref（默认 master）
#   CUDA_ARCHS         CMAKE_CUDA_ARCHITECTURES。优先级：环境变量 > nvidia-smi 自动探测。
#                      未设且无 nvidia-smi 时，脚本拒绝继续。常见值：
#                          75  - Turing  (RTX 20xx, T4)
#                          80  - Ampere  (A100)
#                          86  - Ampere  (RTX 30xx, RTX A4000/A5000/A6000, RTX 3050 Laptop)
#                          89  - Ada     (RTX 40xx, L4)
#                          90  - Hopper  (H100, H200)
#                          120 - Blackwell (RTX 50xx, B100/B200)
#                      多 arch 用分号分隔，如 "86;89"
#   CUDA_PKG           apt 安装的 toolkit 包名。默认空 → 由 recommend_cuda_pkg() 自动选：
#                          Blackwell (sm_100/120) 必须 cuda-toolkit-13-2 (driver ≥ 580)
#                          其它 GPU 默认 cuda-toolkit-12-6 (体积小、向下兼容)
#                          driver < 580 自动从 13.x 降到 12-6
#                      ★ WSL2 必须用 cuda-toolkit-* 而不是 cuda-* —— 后者会装 driver 把
#                        WSL2 GPU 直通搞坏。手动覆盖时记得只写 toolkit 包名。
#   NVIDIA_DISTRO      NVIDIA 仓库 distro tag。未设时按 /etc/os-release 自动探测：
#                          debian 13 → debian13     debian 12 → debian12
#                          ubuntu 24.04 → ubuntu2404  ubuntu 22.04 → ubuntu2204
#                      其它发行版会 die 并提示。
#   JOBS               并行编译核数（默认 nproc）
#   ASSUME_YES         非空时跳过所有交互确认
#   SKIP_CUDA_INSTALL  非空时跳过 toolkit 安装（仅做编译）
#   SKIP_DEPS          非空时跳过 cmake/ninja/curl 依赖安装
#   ALLOW_SHA1_KEYRING 兼容旧脚本，默认 "auto"（不再做全局 SHA1 降级）。
#                      Debian 13 的 sqv 拒 ubuntu2204 仓库 SHA1 keyring 问题已通过
#                      keep_only_active_distro_source() 解决。

set -euo pipefail

# ---------- 通用 distro 探测 ----------
detect_nvidia_distro() {
  # 探测当前发行版，映射到 NVIDIA CUDA 仓库的 distro tag
  if [[ ! -r /etc/os-release ]]; then
    echo ""; return
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  local id="${ID:-}" ver="${VERSION_ID:-}"
  case "$id:$ver" in
    debian:13*) echo "debian13" ;;
    debian:12*) echo "debian12" ;;
    ubuntu:24.04|ubuntu:24.10) echo "ubuntu2404" ;;
    ubuntu:22.04) echo "ubuntu2204" ;;
    ubuntu:20.04) echo "ubuntu2004" ;;
    *) echo "" ;;
  esac
}

# ---------- 配置 ----------
LLAMA_SRC="${LLAMA_SRC:-$HOME/src/llama.cpp}"
LLAMA_REF="${LLAMA_REF:-master}"
# CUDA_PKG 默认空 → 由 recommend_cuda_pkg() 在 detect_env 后按 GPU arch + driver 版本自动选；
# 想固定写死可在外部 export，例如 CUDA_PKG=cuda-toolkit-12-6
CUDA_PKG="${CUDA_PKG:-}"
NVIDIA_DISTRO="${NVIDIA_DISTRO:-$(detect_nvidia_distro)}"
JOBS="${JOBS:-$(nproc)}"
ASSUME_YES="${ASSUME_YES:-}"
SKIP_CUDA_INSTALL="${SKIP_CUDA_INSTALL:-}"
SKIP_DEPS="${SKIP_DEPS:-}"
# 已废弃: Debian 13 + NVIDIA SHA1 keyring 问题改由 keep_only_active_distro_source() 解决，
# 不再需要全局把 apt 切到 gpg。该变量保留只为向后兼容；显式设非 auto 才会触发老逻辑。
ALLOW_SHA1_KEYRING="${ALLOW_SHA1_KEYRING-auto}"
STAGE="${STAGE:-all}"

if [[ -z "$NVIDIA_DISTRO" ]]; then
  echo "[llama-cuda ERR] 无法自动探测 NVIDIA distro tag，请显式设置，例如：" >&2
  echo "  NVIDIA_DISTRO=ubuntu2204  bash $0   # Ubuntu 22.04 WSL" >&2
  echo "  NVIDIA_DISTRO=ubuntu2404  bash $0   # Ubuntu 24.04 WSL" >&2
  echo "  NVIDIA_DISTRO=debian13    bash $0   # Debian 13 trixie" >&2
  exit 1
fi

CUDA_KEYRING_URL="https://developer.download.nvidia.com/compute/cuda/repos/${NVIDIA_DISTRO}/x86_64/cuda-keyring_1.1-1_all.deb"

# ---------- 工具函数 ----------
log()  { printf '\033[1;36m[llama-cuda]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[llama-cuda WARN]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[llama-cuda ERR]\033[0m %s\n' "$*" >&2; exit 1; }

confirm() {
  # confirm "提示" → 非交互模式下自动 yes
  local msg="$1"
  if [[ -n "$ASSUME_YES" ]]; then
    log "[auto-yes] $msg"
    return 0
  fi
  read -rp "$msg [y/N] " ans
  [[ "$ans" =~ ^[Yy]$ ]]
}

stage_enabled() {
  local s="$1"
  [[ "$STAGE" == "all" || "$STAGE" == "$s" ]]
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"
}

# ---------- 探测环境 ----------

# 给定 VRAM (MiB)，打印一句"能跑什么模型"建议
suggest_models_for_vram() {
  local vram_mb=$1
  if   (( vram_mb >= 22000 )); then echo "≥22GB: 70B Q4_K_M 全 offload / 32B Q5_K_M / 9B 全 ctx 32K"
  elif (( vram_mb >= 14000 )); then echo "≥14GB: 32B Q4_K_M 全 offload / 14B Q5_K_M / 9B Q4 ctx 16K"
  elif (( vram_mb >=  7500 )); then echo "≥8GB:  9B Q4_K_M 全 offload (ctx 4K) / 14B Q4 ngl 28~30 / 7B 全 ctx 16K"
  elif (( vram_mb >=  5500 )); then echo "≥6GB:  7B Q4_K_M 全 offload (ctx 4K) / 9B Q4 ngl ~22"
  elif (( vram_mb >=  3500 )); then echo "≥4GB:  4B Q4_K_M 全 offload / 7B Q4 ngl ~18~22 (混合) / 不建议 9B+"
  else                              echo "<4GB:  仅 1.5~3B Q4_K_M 全 offload；其余必须 ngl<full + RAM 充足"
  fi
}

# 给定 CUDA_ARCHS（"86" 或 "86;89"），返回 archs 中最大值（整数）
max_cuda_arch() {
  printf '%s\n' "$1" | tr ';' '\n' | sort -n | tail -1
}

# 比较版本 a >= b（仅看主.次,例如 580.65 vs 580 → 1）
ver_ge() {
  awk -v a="$1" -v b="$2" 'BEGIN{
    split(a,x,".");split(b,y,".")
    for(i=1;i<=2;i++){xi=x[i]+0;yi=y[i]+0;if(xi>yi){print 1;exit}else if(xi<yi){print 0;exit}}
    print 1
  }'
}

# 根据 GPU arch + driver 版本，推荐合适的 cuda-toolkit-* 包
# 规则（参考 https://docs.nvidia.com/cuda/cuda-toolkit-release-notes/）:
#   - Blackwell (sm_100/120) 必须 CUDA 13.0+        → cuda-toolkit-13-2
#   - Hopper   (sm_90)       推荐 12.3+，13.x 也行  → 13-2 或 12-6
#   - Ada      (sm_89)       推荐 12.x+             → 12-6 (体积小，向下兼容好)
#   - Ampere   (sm_80/86)    推荐 12.x              → 12-6
#   - Turing   (sm_75)       推荐 12.x              → 12-6
# CUDA 13.x 严格要求 driver ≥ 580；12.x 要求 ≥ 525。driver 不够就降级。
recommend_cuda_pkg() {
  local archs=$1 driver=$2 max
  max=$(max_cuda_arch "$archs")
  local want="cuda-toolkit-12-6"
  # Blackwell 强制 13.x（更早不支持 sm_100/120）
  if (( max >= 100 )); then want="cuda-toolkit-13-2"; fi
  # 检查 driver 兼容性，不够就降级或拒绝
  if [[ -n "$driver" && "$driver" != "unknown" ]]; then
    case "$want" in
      cuda-toolkit-13-*)
        if [[ "$(ver_ge "$driver" 580)" != 1 ]]; then
          if (( max >= 100 )); then
            warn "driver=$driver < 580：CUDA 13.x 装上也跑不动 sm_${max} (Blackwell 必须 driver ≥ 580)。"
            warn "请在 Windows 端升级 NVIDIA driver 到 ≥ 580 再继续，否则编译/运行会失败。"
          else
            warn "driver=$driver < 580：自动从 13.2 降级到 12.6（更兼容老 driver）"
            want="cuda-toolkit-12-6"
          fi
        fi
        ;;
      cuda-toolkit-12-*)
        if [[ "$(ver_ge "$driver" 525)" != 1 ]]; then
          warn "driver=$driver < 525：CUDA 12.x 也可能跑不动，建议 Windows 端升级 driver"
        fi
        ;;
    esac
  fi
  echo "$want"
}

# 只校验 NVIDIA_DISTRO 跟 OS 是否匹配；不强制改写
check_distro_consistency() {
  local detected
  detected=$(detect_nvidia_distro)
  if [[ -n "$detected" && "$detected" != "$NVIDIA_DISTRO" ]]; then
    warn "NVIDIA_DISTRO=$NVIDIA_DISTRO 与本机 distro 探测结果 '$detected' 不一致。"
    warn "如果不确定，建议改用 NVIDIA_DISTRO=$detected"
  fi
}

detect_env() {
  log "uname: $(uname -srm)  distro=$NVIDIA_DISTRO"
  check_distro_consistency
  local in_wsl=0
  if grep -qi microsoft /proc/version 2>/dev/null; then
    in_wsl=1
    log "运行于 WSL2（不会安装 NVIDIA 驱动；驱动来自 Windows 宿主，与 interop 开关无关）"
  fi

  local has_nvsmi=0 driver="unknown" vram_mb=0 gpu_name=""
  if command -v nvidia-smi >/dev/null 2>&1; then
    has_nvsmi=1
    nvidia-smi --query-gpu=name,driver_version,compute_cap,memory.total \
               --format=csv,noheader \
      | sed 's/^/  GPU: /'
    driver=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 | awk '{print $1}')
    vram_mb=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits 2>/dev/null \
              | head -1 | awk '{print int($1)}')
    gpu_name=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -1 | sed 's/^[ \t]*//')
  elif [[ $in_wsl -eq 1 && -e /usr/lib/wsl/lib/libcuda.so.1 ]]; then
    warn "未找到 nvidia-smi，但 /usr/lib/wsl/lib/libcuda.so.1 存在 → WSL2 GPU 直通已启用。"
    warn "首次安装 cuda-toolkit-* 后才会出现 /usr/local/cuda/bin/nvidia-smi。"
    warn "请显式设置 CUDA_ARCHS（脚本不会替你猜，错值会让编出来的二进制不可用）。"
  elif [[ $in_wsl -eq 1 ]]; then
    die "WSL 内没有 /usr/lib/wsl/lib/libcuda.so.1。\n\
请在 Windows 宿主装最新 NVIDIA Game Ready / Studio 驱动 (≥ 535)，然后 'wsl --shutdown' 重启 WSL。"
  else
    die "未找到 nvidia-smi。原生 Linux 请先装 NVIDIA 驱动并 reboot。"
  fi

  # ---- CUDA_ARCHS 自动 / 显式 ----
  local archs_source="env"
  if [[ -z "${CUDA_ARCHS:-}" ]]; then
    archs_source="nvidia-smi"
    if [[ $has_nvsmi -eq 1 ]]; then
      # 把 "12.0" 转成 "120"；多 GPU 取并集
      local raw archs
      raw=$(nvidia-smi --query-gpu=compute_cap --format=csv,noheader 2>/dev/null || true)
      archs=$(printf '%s\n' "$raw" | awk -F. 'NF==2 {print $1$2}' | sort -u | paste -sd';' -)
      [[ -n "$archs" ]] && CUDA_ARCHS="$archs"
    fi
    if [[ -z "${CUDA_ARCHS:-}" ]]; then
      die "无法自动探测 CUDA_ARCHS，请显式提供。常见取值：
    CUDA_ARCHS=75   bash $0   # Turing  (RTX 20xx, T4)
    CUDA_ARCHS=80   bash $0   # Ampere  (A100)
    CUDA_ARCHS=86   bash $0   # Ampere  (RTX 30xx, RTX 3050 Laptop, A4000/5000/6000)
    CUDA_ARCHS=89   bash $0   # Ada     (RTX 40xx, L4)
    CUDA_ARCHS=90   bash $0   # Hopper  (H100, H200)
    CUDA_ARCHS=120  bash $0   # Blackwell (RTX 50xx, B100/B200)
    CUDA_ARCHS='86;89' bash $0  # 多 arch 同时构建"
    fi
  fi

  # ---- CUDA_PKG 自动 / 显式 ----
  local pkg_source="env"
  if [[ -z "$CUDA_PKG" ]]; then
    pkg_source="auto"
    CUDA_PKG=$(recommend_cuda_pkg "$CUDA_ARCHS" "$driver")
  fi

  # ---- 已装组件预检 ----
  local nvcc_ver="(未装)" cmake_ver="(未装)" ninja_ver="(未装)"
  if command -v nvcc >/dev/null 2>&1; then
    nvcc_ver=$(nvcc --version 2>/dev/null | grep -oE 'release [0-9]+\.[0-9]+' | head -1)
    [[ -z "$nvcc_ver" ]] && nvcc_ver="(已装,未识别版本)"
  fi
  command -v cmake >/dev/null 2>&1 && cmake_ver=$(cmake --version 2>/dev/null | head -1 | awk '{print $3}')
  command -v ninja >/dev/null 2>&1 && ninja_ver=$(ninja --version 2>/dev/null)
  local src_state="(未拉取)"
  [[ -d "$LLAMA_SRC/.git" ]] && src_state="$(git -C "$LLAMA_SRC" rev-parse --short HEAD 2>/dev/null) ($LLAMA_SRC)"
  local bin_state="(未编译)"
  [[ -x "$LLAMA_SRC/build/bin/llama-cli" ]] && bin_state="✓ $LLAMA_SRC/build/bin/llama-cli"

  # ---- 决策汇总（关键!）----
  echo
  echo "============================================================"
  echo "[决策汇总] llama.cpp + CUDA 编译方案"
  echo "============================================================"
  printf "  GPU            : %s\n"  "${gpu_name:-(无 nvidia-smi)}"
  printf "  driver_version : %s\n"  "$driver"
  printf "  VRAM           : %s MiB%s\n"  "$vram_mb" \
    "$([[ $vram_mb -gt 0 ]] && printf '  → %s' "$(suggest_models_for_vram "$vram_mb")")"
  printf "  distro         : %s\n"  "$NVIDIA_DISTRO"
  printf "  CUDA_ARCHS     : %s   (来源: %s)\n"  "$CUDA_ARCHS" "$archs_source"
  printf "  CUDA_PKG       : %s   (来源: %s)\n"  "$CUDA_PKG" "$pkg_source"
  printf "  nvcc           : %s\n"  "$nvcc_ver"
  printf "  cmake          : %s\n"  "$cmake_ver"
  printf "  ninja          : %s\n"  "$ninja_ver"
  printf "  llama.cpp src  : %s\n"  "$src_state"
  printf "  编译产物       : %s\n"  "$bin_state"
  printf "  并行编译       : -j%s\n" "$JOBS"
  printf "  STAGE          : %s\n"  "$STAGE"
  echo "------------------------------------------------------------"
  echo "  覆盖任意决策: 在命令前 export，例如:"
  echo "      CUDA_PKG=cuda-toolkit-12-6 CUDA_ARCHS=86 NVIDIA_DISTRO=ubuntu2204 \\"
  echo "        bash $0"
  echo "============================================================"
  echo
}

# ---------- 清理 NVIDIA 仓库 sources（Debian 13+ 必需） ----------
# 背景：cuda-keyring 1.1-1 会同时注入 cuda-debian13-* 和 cuda-ubuntu2204-* 两个 source。
# Debian 13 的 sqv 拒绝 ubuntu2204 仓库使用的 SHA1 keyring（见
# https://github.com/NVIDIA/cuda-repo-management/issues/34），导致 apt-get update 报错
# 并以非零退出。debian13 仓库 InRelease 校验是通过的，所以正确处置是只保留它、禁用其它。
keep_only_active_distro_source() {
  local active_basename="cuda-${NVIDIA_DISTRO}-x86_64.list"
  shopt -s nullglob
  local f changed=0
  for f in /etc/apt/sources.list.d/cuda-*.list; do
    local base
    base=$(basename "$f")
    [[ "$base" == "$active_basename" ]] && continue
    log "禁用多余 NVIDIA source: $f -> $f.disabled"
    sudo mv "$f" "$f.disabled"
    changed=1
  done
  shopt -u nullglob
  [[ $changed -eq 1 ]] && log "已清理。仅保留 $active_basename"
}

# 兼容老 ALLOW_SHA1_KEYRING：若用户显式设过，且不是 auto，则保留之前那套全局降级
# （只对 apt-key/gpg 工具有意义；apt 自身的 sqv 不受影响，因此本脚本不再依赖它）。
apply_legacy_sha1_workaround_if_requested() {
  [[ -n "$ALLOW_SHA1_KEYRING" && "$ALLOW_SHA1_KEYRING" != "auto" ]] || return 0
  local conf=/etc/apt/apt.conf.d/99-nvidia-sha1-workaround.conf
  if [[ -f "$conf" ]] && grep -q 'weak-digest SHA1' /etc/gnupg/gpg.conf 2>/dev/null; then
    return 0
  fi
  warn "ALLOW_SHA1_KEYRING 非 auto，写入兼容性 workaround（不影响 sqv，可选）"
  echo 'APT::Key::GPGCommand "/usr/bin/gpg";' | sudo tee "$conf" >/dev/null
  sudo install -d /etc/gnupg
  if ! sudo grep -q '^weak-digest SHA1$' /etc/gnupg/gpg.conf 2>/dev/null; then
    echo 'weak-digest SHA1' | sudo tee -a /etc/gnupg/gpg.conf >/dev/null
  fi
}

# ---------- 阶段 1: CUDA Toolkit ----------
install_cuda_toolkit() {
  stage_enabled "cuda" || return 0
  if [[ -n "$SKIP_CUDA_INSTALL" ]]; then
    log "SKIP_CUDA_INSTALL 非空，跳过 toolkit 安装"
    return 0
  fi
  if command -v nvcc >/dev/null 2>&1; then
    log "nvcc 已存在: $(nvcc --version | tail -1)（跳过 toolkit 安装）"
    return 0
  fi

  confirm "未检测到 nvcc，是否用 apt 安装 ${CUDA_PKG}（约 3-5 GB）？" \
    || die "用户取消 CUDA toolkit 安装"

  require_cmd sudo
  require_cmd dpkg

  # 已装 cuda-keyring 直接跳过下载
  if dpkg-query -W -f='${Status}\n' cuda-keyring 2>/dev/null | grep -q '^install ok installed'; then
    log "cuda-keyring 已装，跳过下载"
  else
    local tmp
    tmp=$(mktemp -d)
    trap "rm -rf '$tmp'" RETURN
    log "下载 cuda-keyring (distro=$NVIDIA_DISTRO)..."
    if command -v curl >/dev/null 2>&1; then
      curl -fsSL --retry 3 -o "$tmp/cuda-keyring.deb" "$CUDA_KEYRING_URL" \
        || die "下载失败: $CUDA_KEYRING_URL  (检查 NVIDIA_DISTRO 是否正确)"
    elif command -v wget >/dev/null 2>&1; then
      wget -q -O "$tmp/cuda-keyring.deb" "$CUDA_KEYRING_URL" \
        || die "下载失败: $CUDA_KEYRING_URL  (检查 NVIDIA_DISTRO 是否正确)"
    else
      die "需要 curl 或 wget 之一来下载 cuda-keyring"
    fi
    sudo dpkg -i "$tmp/cuda-keyring.deb"
  fi

  keep_only_active_distro_source
  apply_legacy_sha1_workaround_if_requested
  log "apt-get update（NVIDIA 仓库）..."
  sudo apt-get update
  # 兼容性检查：包是否真的存在于当前 distro 仓库
  if ! apt-cache show "$CUDA_PKG" >/dev/null 2>&1; then
    die "$CUDA_PKG 在 ${NVIDIA_DISTRO} 仓库里不存在。可选:
  - 浏览 https://developer.download.nvidia.com/compute/cuda/repos/${NVIDIA_DISTRO}/x86_64/ 看可用版本
  - 用 'apt-cache search cuda-toolkit-' 看本地已识别的版本
  - 改 CUDA_PKG=cuda-toolkit-12-6 之类已知存在的版本"
  fi
  log "安装 ${CUDA_PKG}（这一步比较慢）..."
  sudo apt-get install -y "$CUDA_PKG"

  if ! grep -q '/usr/local/cuda/bin' "$HOME/.bashrc" 2>/dev/null; then
    log "把 /usr/local/cuda/bin 写入 ~/.bashrc"
    {
      echo ''
      echo '# Added by build_llamacpp_cuda.sh'
      echo 'export PATH=/usr/local/cuda/bin:$PATH'
      echo 'export LD_LIBRARY_PATH=/usr/local/cuda/lib64:${LD_LIBRARY_PATH:-}'
    } >> "$HOME/.bashrc"
  fi
  export PATH=/usr/local/cuda/bin:$PATH
  export LD_LIBRARY_PATH=/usr/local/cuda/lib64:${LD_LIBRARY_PATH:-}
  command -v nvcc >/dev/null 2>&1 \
    || die "nvcc 安装后仍找不到，检查 /usr/local/cuda 软链是否存在"
  log "nvcc OK: $(nvcc --version | tail -1)"
}

# ---------- 阶段 2: 构建依赖 ----------
install_build_deps() {
  stage_enabled "deps" || return 0
  if [[ -n "$SKIP_DEPS" ]]; then
    log "SKIP_DEPS 非空，跳过依赖安装"
    return 0
  fi
  local missing=()
  for c in cmake ninja git pkg-config curl; do
    command -v "$c" >/dev/null 2>&1 || missing+=("$c")
  done
  if [[ ${#missing[@]} -eq 0 ]]; then
    log "构建依赖齐全（cmake/ninja/git/pkg-config/curl）"
    return 0
  fi

  confirm "缺少构建依赖：${missing[*]}，是否 apt 安装 cmake/ninja-build/git/pkg-config/libcurl4-openssl-dev/libssl-dev？" \
    || die "用户取消依赖安装"
  require_cmd sudo
  sudo apt-get install -y \
    cmake ninja-build ccache git pkg-config \
    libcurl4-openssl-dev libssl-dev
}

# ---------- 阶段 3: 拉源码 ----------
fetch_source() {
  stage_enabled "fetch" || return 0
  mkdir -p "$(dirname "$LLAMA_SRC")"
  if [[ -d "$LLAMA_SRC/.git" ]]; then
    log "更新已有源码: $LLAMA_SRC"
    git -C "$LLAMA_SRC" fetch --tags --prune origin
    git -C "$LLAMA_SRC" checkout "$LLAMA_REF"
    git -C "$LLAMA_SRC" pull --ff-only origin "$LLAMA_REF" || warn "pull 失败（可能是 detached HEAD），继续。"
  else
    log "克隆 llama.cpp 到 $LLAMA_SRC"
    git clone https://github.com/ggml-org/llama.cpp "$LLAMA_SRC"
    git -C "$LLAMA_SRC" checkout "$LLAMA_REF"
  fi
  log "当前 commit: $(git -C "$LLAMA_SRC" rev-parse --short HEAD)"
}

# ---------- 阶段 4: 编译 ----------
build_llama() {
  stage_enabled "build" || return 0
  [[ -d "$LLAMA_SRC" ]] || die "源码目录不存在: $LLAMA_SRC（先跑 STAGE=fetch）"
  command -v nvcc >/dev/null 2>&1 \
    || die "未找到 nvcc；先 STAGE=cuda 或 source ~/.bashrc"
  command -v cmake >/dev/null 2>&1 \
    || die "未找到 cmake；先 STAGE=deps"

  cd "$LLAMA_SRC"
  log "cmake configure (CUDA archs=$CUDA_ARCHS, jobs=$JOBS)"
  cmake -B build -G Ninja \
    -DGGML_CUDA=ON \
    -DCMAKE_CUDA_ARCHITECTURES="$CUDA_ARCHS" \
    -DGGML_NATIVE=ON \
    -DLLAMA_CURL=ON \
    -DCMAKE_BUILD_TYPE=Release

  log "cmake build (这一步耗时 5-30 分钟，取决于 CPU 和 ccache)"
  cmake --build build --config Release -j "$JOBS"

  log "构建完成。关键产物："
  for b in llama-cli llama-server llama-bench llama-completion; do
    [[ -x "build/bin/$b" ]] && printf '  ✓ build/bin/%s\n' "$b"
  done
}

# ---------- 阶段 5: 验证 ----------
verify() {
  stage_enabled "verify" || return 0
  local bin="$LLAMA_SRC/build/bin/llama-cli"
  [[ -x "$bin" ]] || die "未找到 $bin（先跑 STAGE=build）"

  log "调用 $bin --list-devices ："
  if "$bin" --list-devices 2>&1 | tee /tmp/llama_cuda_devices.txt | grep -qi 'CUDA'; then
    log "✓ CUDA 后端已加载"
  else
    warn "未在 --list-devices 输出里看到 CUDA。完整输出 → /tmp/llama_cuda_devices.txt"
    warn "常见原因："
    warn "  1) CUDA_ARCHS 与 GPU 不匹配（编出来的 cubin 跑不起来）"
    warn "  2) WSL2 GPU 直通未生效：Windows 端 nvidia-smi.exe 应能看到设备"
    warn "  3) /usr/lib/wsl/lib/libcuda.so.1 缺失：升级 NVIDIA driver + 'wsl --shutdown'"
  fi

  # 取一次 VRAM，给出有针对性的下一步建议
  local vram_mb=0
  if command -v nvidia-smi >/dev/null 2>&1; then
    vram_mb=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits 2>/dev/null \
              | head -1 | awk '{print int($1)}')
  fi

  cat <<EOF

============================================================
下一步：让新二进制优先（可选；只在你装过 brew/系统的 llama.cpp 时才需要）
  echo 'export PATH=$LLAMA_SRC/build/bin:\$PATH' >> ~/.bashrc
  source ~/.bashrc
  which llama-cli   # 应当指向 $LLAMA_SRC/build/bin/llama-cli

GPU 基准（替换 GGUF 路径）：
  $LLAMA_SRC/build/bin/llama-bench \\
      -m <path/to/model.gguf> -ngl 99 -p 256 -n 128
============================================================
EOF

  if [[ "$vram_mb" -gt 0 ]]; then
    cat <<EOF
本机 VRAM = ${vram_mb} MiB → $(suggest_models_for_vram "$vram_mb")
EOF
    if (( vram_mb < 5500 )); then
      cat <<'EOF'

★ 显存 < 6 GB 实战建议（适用于 RTX 3050 4GB Laptop 等）：
  1. 优先选 4B / 7B Q4_K_M GGUF (例: Qwen2.5-3B / Qwen2.5-7B)。
  2. 跑 9B 必须混合模式：-ngl 18~22 + -c 2048（实测公式见
     docs/benchmarks/qwen35-9b-llamacpp-vs-ollama.md §4.3）。
  3. 不要并行 Ollama 与 llama-server：8GB 卡都顶不住，4GB 必撕裂。
EOF
    fi
  fi

  cat <<EOF

(更多用法见 docs/framework/llamacpp-cheatsheet.md 与
 docs/benchmarks/qwen35-9b-llamacpp-vs-ollama.md)
EOF
}

# ---------- 入口 ----------
main() {
  log "STAGE=$STAGE  LLAMA_SRC=$LLAMA_SRC  LLAMA_REF=$LLAMA_REF"
  detect_env
  install_cuda_toolkit
  install_build_deps
  fetch_source
  build_llama
  verify
  log "全部完成"
}

main "$@"

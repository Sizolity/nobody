#!/usr/bin/env bash
# recommend_llamacpp_yaml.sh — print a recommended nobody.yaml diff +
# matching `make llamacpp-up` ENV command for the current machine.
#
# Detects:
#   - nvidia-smi presence + first GPU's total VRAM (MiB).
# Prints:
#   - A YAML snippet (model.name + runtime.num_ctx) operator can paste
#     into nobody.yaml.
#   - A `make llamacpp-up` invocation with ENV vars filled in.
#
# Does NOT modify any file, does NOT call sudo, does NOT write to
# nobody.yaml. Pure stdout.
#
# NOTE (2026-04-28, Phase 2): the source of truth for the auto-recommendation
# decision matrix is now internal/inference/llamacpp/recommend.go (Go port,
# called inline by harness.New when provider_opts.llamacpp.lifecycle=managed).
# This bash script is kept as a manual diagnose tool — operators run it on a
# fresh machine to print yaml + ENV before they wire managed mode. If you
# change the decision matrix here, also update recommend.go (and vice versa);
# CI does NOT enforce parity.

set -euo pipefail

# Decision matrix derived from
# Prints llama.cpp config suggestions for the seed repository.
recommend_for_vram() {
  local vram_mib="$1"
  local model ngl ctx note

  if (( vram_mib >= 24576 )); then
    model="qwen3.5-27b"
    ngl="99"
    ctx="32768"
    note="≥24 GB VRAM: full GPU offload of 27B with large ctx"
  elif (( vram_mib >= 16384 )); then
    model="qwen3.5-9b"
    ngl="99"
    ctx="32768"
    note="16-23 GB VRAM: full GPU offload of 9B with large ctx"
  elif (( vram_mib >= 6144 )); then
    model="qwen3.5-9b"
    ngl="99"
    ctx="8192"
    note="6-15 GB VRAM: full GPU offload of 9B at safe ctx (32K would OOM per benchmark)"
  elif (( vram_mib >= 4096 )); then
    model="qwen3.5-9b"
    ngl="22"
    ctx="4096"
    note="4-5 GB VRAM: hybrid offload of 9B (CPU partially involved)"
  else
    model="qwen3.5-4b"
    ngl="99"
    ctx="8192"
    note="<4 GB VRAM: 4B fits fully on GPU"
  fi

  cat <<RECOMMEND
# Detected: GPU with ${vram_mib} MiB total VRAM
# Recommendation: ${note}
#
# nobody.yaml diff:
model:
  name: ${model}
runtime:
  num_ctx: ${ctx}
#
# Start server with:
LLAMA_NGL=${ngl} \\
  LLAMA_CHAT_MODEL=\$HOME/models/${model}-q4_k_m.gguf \\
  LLAMA_CHAT_CTX=${ctx} \\
  make llamacpp-up
RECOMMEND
}

recommend_cpu_only() {
  cat <<RECOMMEND
# Detected: no nvidia-smi on PATH (CPU-only or non-NVIDIA GPU)
# Recommendation: pure CPU inference of qwen3.5-4b
#
# nobody.yaml diff:
model:
  name: qwen3.5-4b
runtime:
  num_ctx: 8192
#
# Start server with (no LLAMA_NGL → CPU only):
LLAMA_CHAT_MODEL=\$HOME/models/qwen3.5-4b-q4_k_m.gguf \\
  LLAMA_CHAT_CTX=8192 \\
  make llamacpp-up
#
# To experiment with GPU offload later, re-run this script after installing
# nvidia-smi (CUDA toolkit) or set LLAMA_NGL=N manually.
RECOMMEND
}

main() {
  echo "# recommend_llamacpp_yaml.sh — generated $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "# WARNING: prints recommendations only; does not modify nobody.yaml."
  echo
  if ! command -v nvidia-smi >/dev/null 2>&1; then
    recommend_cpu_only
    return 0
  fi

  # Take only the first GPU's total memory (multi-GPU users typically
  # already know what they want; we keep this script simple).
  local vram_line
  vram_line="$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits 2>/dev/null | head -n 1 || true)"
  if [[ -z "$vram_line" ]]; then
    echo "# nvidia-smi present but VRAM query failed; falling back to CPU recommendation."
    echo
    recommend_cpu_only
    return 0
  fi
  # Strip any non-digit chars (just in case).
  local vram_mib="${vram_line//[^0-9]/}"
  if [[ -z "$vram_mib" ]]; then
    echo "# nvidia-smi returned non-numeric VRAM ($vram_line); falling back to CPU recommendation."
    echo
    recommend_cpu_only
    return 0
  fi

  recommend_for_vram "$vram_mib"
}

main "$@"

#!/usr/bin/env bash
# llamacpp-server.sh — sidecar to start/stop external llama-server
# instances for nobody's llamacpp provider (openai_compat mode).
#
# State (PID files + logs) lives under $XDG_STATE_HOME/nobody/llamacpp
# (default: ~/.local/state/nobody/llamacpp/) so this script does NOT
# pollute the workspace.
#
# NOTE (2026-04-28, Phase 2): nobody now also supports an in-process
# "managed" lifecycle that forks llama-server from harness.New (see
# provider_opts.llamacpp.lifecycle in nobody.yaml). Sidecar (this script)
# and managed coexist via /health-200 reuse: if you start this script
# first and then run harness with lifecycle=managed, harness REUSES this
# process and does NOT kill it on Close (Phase 2 invariant #4).
#
# Sidecar remains the right tool for: multi-host deployments, CI
# pre-warming, debugging llama-server config independent of harness, and
# any setup where the model server must outlive the harness process. See
# Kept as a local llama.cpp helper for the Narrative Harness seed repository.
# §1.2 / §2.5.
#
# Defaults assume a CPU-only llama-server install (brew install
# llama.cpp / apt install). To enable GPU offload, export LLAMA_NGL=99
# (or a custom layer count). All other parameters are overrideable via
# ENV; see the table below.
#
# ENV (with defaults):
#   LLAMA_BIN=llama-server                                 # PATH lookup
#   LLAMA_HOST=0.0.0.0                                     # bind addr; set 127.0.0.1 to restrict to loopback
#   LLAMA_CHAT_MODEL=$HOME/models/qwen3.5-4b-q4_k_m.gguf
#   LLAMA_CHAT_PORT=8080
#   LLAMA_CHAT_CTX=8192
#   LLAMA_CHAT_TEMPLATE=qwen3
#   LLAMA_NGL=                                             # empty = pure CPU
#   LLAMA_EXTRA_FLAGS=
#
#   LLAMA_EMBED_ENABLED=0                                  # 1 to start
#   LLAMA_EMBED_MODEL=$HOME/models/nomic-embed-text-v1.5.gguf
#   LLAMA_EMBED_PORT=8081
#
# Usage:
#   scripts/llamacpp-server.sh up
#   scripts/llamacpp-server.sh down
#   scripts/llamacpp-server.sh status
#   scripts/llamacpp-server.sh restart

set -euo pipefail

LLAMA_BIN="${LLAMA_BIN:-llama-server}"
LLAMA_HOST="${LLAMA_HOST:-0.0.0.0}"
LLAMA_CHAT_MODEL="${LLAMA_CHAT_MODEL:-$HOME/models/qwen3.5-4b-q4_k_m.gguf}"
LLAMA_CHAT_PORT="${LLAMA_CHAT_PORT:-8080}"
LLAMA_CHAT_CTX="${LLAMA_CHAT_CTX:-8192}"
LLAMA_CHAT_TEMPLATE="${LLAMA_CHAT_TEMPLATE:-qwen3}"
LLAMA_NGL="${LLAMA_NGL:-}"
LLAMA_EXTRA_FLAGS="${LLAMA_EXTRA_FLAGS:-}"

LLAMA_EMBED_ENABLED="${LLAMA_EMBED_ENABLED:-0}"
LLAMA_EMBED_MODEL="${LLAMA_EMBED_MODEL:-$HOME/models/nomic-embed-text-v1.5.gguf}"
LLAMA_EMBED_PORT="${LLAMA_EMBED_PORT:-8081}"

STATE_HOME="${XDG_STATE_HOME:-$HOME/.local/state}/nobody/llamacpp"
mkdir -p "$STATE_HOME"

CHAT_PID_FILE="$STATE_HOME/chat.pid"
CHAT_LOG_FILE="$STATE_HOME/chat.log"
EMBED_PID_FILE="$STATE_HOME/embed.pid"
EMBED_LOG_FILE="$STATE_HOME/embed.log"

# ── helpers ────────────────────────────────────────────────────────────

log() { printf '[llamacpp-server] %s\n' "$*"; }
die() { printf '[llamacpp-server] error: %s\n' "$*" >&2; exit 1; }

pid_alive() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

read_pid() {
  local file="$1"
  [[ -f "$file" ]] && cat "$file" 2>/dev/null || true
}

cleanup_stale_pid() {
  local file="$1"
  local pid
  pid="$(read_pid "$file")"
  if [[ -n "$pid" ]] && ! pid_alive "$pid"; then
    log "removing stale pid file $file (pid $pid not alive)"
    rm -f "$file"
  fi
}

port_in_use() {
  local port="$1"
  # Prefer ss (iproute2); fall back to lsof when ss is not available.
  if command -v ss >/dev/null 2>&1; then
    ss -lnt "sport = :$port" 2>/dev/null | tail -n +2 | grep -q .
  elif command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN -nP >/dev/null 2>&1
  else
    # No tooling — assume free; will fail at llama-server startup if not.
    return 1
  fi
}

wait_health() {
  local port="$1" name="$2"
  # llama-server bind 到 0.0.0.0 时探针走 127.0.0.1；其它显式 bind（如 127.0.0.1
  # 或具体网卡 IP）保持原值，避免假阴性。
  local probe_host="$LLAMA_HOST"
  if [[ "$probe_host" == "0.0.0.0" || -z "$probe_host" ]]; then
    probe_host="127.0.0.1"
  fi
  local url="http://$probe_host:$port/health"
  for _ in $(seq 1 30); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$name health probe ok at $url"
      return 0
    fi
    sleep 1
  done
  die "$name failed to respond on /health within 30s; see $STATE_HOME/${name}.log"
}

# ── up: start a single server ──────────────────────────────────────────

start_server() {
  local name="$1" model="$2" port="$3" pid_file="$4" log_file="$5"; shift 5
  local extra_args=("$@")

  cleanup_stale_pid "$pid_file"
  local existing
  existing="$(read_pid "$pid_file")"
  if pid_alive "$existing"; then
    log "$name already running (pid $existing, port $port); skipping"
    return 0
  fi
  if port_in_use "$port"; then
    die "port $port already bound by another process; not killing — set LLAMA_${name^^}_PORT to a free port or stop the other process"
  fi
  if ! command -v "$LLAMA_BIN" >/dev/null 2>&1; then
    die "LLAMA_BIN=$LLAMA_BIN not found in PATH; install llama.cpp (brew install llama.cpp / apt install llama.cpp / scripts/build_llamacpp_cuda.sh) or set LLAMA_BIN to the full path"
  fi
  if [[ ! -f "$model" ]]; then
    die "model file not found: $model — download a GGUF (e.g. huggingface-cli download Qwen/Qwen2.5-4B-Instruct-GGUF) and set LLAMA_${name^^}_MODEL"
  fi

  log "starting $name on port $port (model: $model)"
  log "  log: $log_file"
  # Use nohup + disown so the process survives the shell exit.
  nohup "$LLAMA_BIN" \
    --model "$model" \
    --host "$LLAMA_HOST" \
    --port "$port" \
    "${extra_args[@]}" \
    > "$log_file" 2>&1 &
  local new_pid=$!
  echo "$new_pid" > "$pid_file"
  disown "$new_pid" 2>/dev/null || true
  wait_health "$port" "$name"
  log "$name ready (pid $new_pid)"
}

cmd_up() {
  # Build chat-server arg vector; only append -ngl when the operator opted in.
  local chat_args=(
    -c "$LLAMA_CHAT_CTX"
    --parallel 1
    --cont-batching
    --chat-template "$LLAMA_CHAT_TEMPLATE"
  )
  if [[ -n "$LLAMA_NGL" ]]; then
    chat_args+=(-ngl "$LLAMA_NGL")
  fi
  if [[ -n "$LLAMA_EXTRA_FLAGS" ]]; then
    # Word-split intentionally so the operator can pass multi-token flags.
    # shellcheck disable=SC2206
    chat_args+=($LLAMA_EXTRA_FLAGS)
  fi
  start_server "chat" "$LLAMA_CHAT_MODEL" "$LLAMA_CHAT_PORT" "$CHAT_PID_FILE" "$CHAT_LOG_FILE" "${chat_args[@]}"

  if [[ "$LLAMA_EMBED_ENABLED" == "1" ]]; then
    local embed_args=(--embedding)
    start_server "embed" "$LLAMA_EMBED_MODEL" "$LLAMA_EMBED_PORT" "$EMBED_PID_FILE" "$EMBED_LOG_FILE" "${embed_args[@]}"
  fi
}

# ── down: stop a single server ─────────────────────────────────────────

stop_server() {
  local name="$1" pid_file="$2"
  cleanup_stale_pid "$pid_file"
  local pid
  pid="$(read_pid "$pid_file")"
  if [[ -z "$pid" ]]; then
    log "$name not running (no pid file)"
    return 0
  fi
  if ! pid_alive "$pid"; then
    log "$name pid $pid not alive; cleaning up"
    rm -f "$pid_file"
    return 0
  fi
  log "stopping $name (pid $pid) via SIGTERM"
  kill -TERM "$pid" 2>/dev/null || true
  for _ in $(seq 1 10); do
    if ! pid_alive "$pid"; then
      rm -f "$pid_file"
      log "$name stopped"
      return 0
    fi
    sleep 1
  done
  log "$name still alive after 10s; sending SIGKILL"
  kill -KILL "$pid" 2>/dev/null || true
  rm -f "$pid_file"
}

cmd_down() {
  stop_server "embed" "$EMBED_PID_FILE"
  stop_server "chat" "$CHAT_PID_FILE"
}

# ── status: report ─────────────────────────────────────────────────────

report_one() {
  local name="$1" pid_file="$2" log_file="$3" port="$4"
  cleanup_stale_pid "$pid_file"
  local pid
  pid="$(read_pid "$pid_file")"
  if [[ -z "$pid" ]] || ! pid_alive "$pid"; then
    printf '  %-6s : no server running\n' "$name"
    return 0
  fi
  local probe_host="$LLAMA_HOST"
  if [[ "$probe_host" == "0.0.0.0" || -z "$probe_host" ]]; then
    probe_host="127.0.0.1"
  fi
  local health="unknown"
  if curl -fsS "http://$probe_host:$port/health" >/dev/null 2>&1; then
    health="healthy"
  else
    health="probe failed"
  fi
  printf '  %-6s : pid=%s port=%s health=%s log=%s\n' "$name" "$pid" "$port" "$health" "$log_file"
  if [[ -f "$log_file" ]]; then
    printf '    last 5 log lines:\n'
    tail -n 5 "$log_file" | sed 's/^/      /'
  fi
}

cmd_status() {
  log "state dir: $STATE_HOME"
  report_one "chat" "$CHAT_PID_FILE" "$CHAT_LOG_FILE" "$LLAMA_CHAT_PORT"
  report_one "embed" "$EMBED_PID_FILE" "$EMBED_LOG_FILE" "$LLAMA_EMBED_PORT"
}

# ── dispatch ───────────────────────────────────────────────────────────

main() {
  local sub="${1:-}"
  case "$sub" in
    up)      cmd_up ;;
    down)    cmd_down ;;
    status)  cmd_status ;;
    restart) cmd_down; cmd_up ;;
    *)
      cat <<USAGE >&2
usage: $0 {up|down|status|restart}

ENV (defaults shown):
  LLAMA_BIN=$LLAMA_BIN
  LLAMA_HOST=$LLAMA_HOST  (set 127.0.0.1 to restrict to loopback)
  LLAMA_CHAT_MODEL=$LLAMA_CHAT_MODEL
  LLAMA_CHAT_PORT=$LLAMA_CHAT_PORT
  LLAMA_CHAT_CTX=$LLAMA_CHAT_CTX
  LLAMA_CHAT_TEMPLATE=$LLAMA_CHAT_TEMPLATE
  LLAMA_NGL=$LLAMA_NGL  (empty = CPU only)
  LLAMA_EXTRA_FLAGS=$LLAMA_EXTRA_FLAGS

  LLAMA_EMBED_ENABLED=$LLAMA_EMBED_ENABLED
  LLAMA_EMBED_MODEL=$LLAMA_EMBED_MODEL
  LLAMA_EMBED_PORT=$LLAMA_EMBED_PORT

  XDG_STATE_HOME / state dir: $STATE_HOME
USAGE
      exit 2
      ;;
  esac
}

main "$@"

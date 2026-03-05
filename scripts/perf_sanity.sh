#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
SOCKET_PATH="$TMP_DIR/k11sd.sock"
SESSION_PATH="$TMP_DIR/session.json"
DAEMON_PID=""
BUDGET_MS="${STARTUP_BUDGET_MS:-800}"

cleanup() {
  if [[ -n "$DAEMON_PID" ]]; then
    kill "$DAEMON_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cd "$ROOT_DIR"

go build -o "$TMP_DIR/k11s" ./cmd/k11s
go build -o "$TMP_DIR/k11sd" ./cmd/k11sd

K11S_SOCKET="$SOCKET_PATH" "$TMP_DIR/k11sd" --socket "$SOCKET_PATH" >/dev/null 2>&1 &
DAEMON_PID=$!

for _ in $(seq 1 50); do
  if [[ -S "$SOCKET_PATH" ]]; then
    break
  fi
  sleep 0.1
done

OUTPUT="$(
  K11SD_PATH="$TMP_DIR/k11sd" \
  K11S_SOCKET="$SOCKET_PATH" \
  K11S_SESSION="$SESSION_PATH" \
  "$TMP_DIR/k11s" debug perf --socket "$SOCKET_PATH" --json-only
)"

TOTAL_MS="$(
  awk -F: '/"totalStartupMs"/ {gsub(/[^0-9]/, "", $2); print $2; exit}' <<<"$OUTPUT"
)"

if [[ -z "$TOTAL_MS" ]]; then
  echo "::warning::perf sanity: unable to parse totalStartupMs"
  echo "$OUTPUT"
  exit 0
fi

if (( TOTAL_MS > BUDGET_MS )); then
  echo "::warning::perf sanity: startup ${TOTAL_MS}ms exceeds advisory budget ${BUDGET_MS}ms"
else
  echo "perf sanity: startup ${TOTAL_MS}ms within advisory budget ${BUDGET_MS}ms"
fi

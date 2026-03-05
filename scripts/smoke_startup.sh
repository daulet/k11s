#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
SOCKET_PATH="$TMP_DIR/k11sd.sock"
SESSION_PATH="$TMP_DIR/session.json"
DAEMON_PID=""

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

if [[ ! -S "$SOCKET_PATH" ]]; then
  echo "smoke test failed: daemon socket did not appear at $SOCKET_PATH" >&2
  exit 1
fi

OUTPUT="$(
  K11SD_PATH="$TMP_DIR/k11sd" \
  K11S_SOCKET="$SOCKET_PATH" \
  K11S_SESSION="$SESSION_PATH" \
  "$TMP_DIR/k11s" debug perf --socket "$SOCKET_PATH" --json-only
)"

if ! grep -q '"totalStartupMs"' <<<"$OUTPUT"; then
  echo "smoke test failed: missing totalStartupMs in debug perf output" >&2
  echo "$OUTPUT" >&2
  exit 1
fi

if ! grep -q '"daemon_connect"' <<<"$OUTPUT"; then
  echo "smoke test failed: missing daemon_connect span" >&2
  echo "$OUTPUT" >&2
  exit 1
fi

echo "startup smoke test passed"

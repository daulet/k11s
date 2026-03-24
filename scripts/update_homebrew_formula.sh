#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version-tag> [tap-dir]" >&2
  echo "example: $0 v0.1.0 ../homebrew-tap" >&2
  exit 1
fi

VERSION_TAG="$1"
if [[ "${VERSION_TAG}" != v* ]]; then
  echo "version tag must start with 'v' (got: ${VERSION_TAG})" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAP_DIR="${2:-${ROOT_DIR}/../homebrew-tap}"
CHECKSUMS_PATH="${ROOT_DIR}/dist/checksums.txt"

if [[ ! -f "${CHECKSUMS_PATH}" ]]; then
  echo "missing ${CHECKSUMS_PATH}; run ./scripts/release_bundles.sh ${VERSION_TAG} first" >&2
  exit 1
fi

INTEL_SHA="$(awk '$2 ~ /k11s-x86_64-apple-darwin\.tar\.gz$/ {print $1}' "${CHECKSUMS_PATH}" | tail -n1)"
ARM_SHA="$(awk '$2 ~ /k11s-aarch64-apple-darwin\.tar\.gz$/ {print $1}' "${CHECKSUMS_PATH}" | tail -n1)"

if [[ -z "${INTEL_SHA}" || -z "${ARM_SHA}" ]]; then
  echo "failed to parse required checksums from ${CHECKSUMS_PATH}" >&2
  exit 1
fi

VERSION="${VERSION_TAG#v}"
REPO="${REPO:-${GITHUB_REPOSITORY:-daulet/k11s}}"
DESC="${DESC:-Speed-first CLI/TUI for Kubernetes navigation and operations}"

python3 "${ROOT_DIR}/.github/scripts/update_homebrew_formula.py" \
  --formula "${TAP_DIR}/Formula/k11s.rb" \
  --repo "${REPO}" \
  --tag "${VERSION_TAG}" \
  --version "${VERSION}" \
  --binary "k11s" \
  --daemon-binary "k11sd" \
  --sha-intel "${INTEL_SHA}" \
  --sha-arm "${ARM_SHA}" \
  --desc "${DESC}"

echo "updated formula: ${TAP_DIR}/Formula/k11s.rb"

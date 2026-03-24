#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <version-tag>" >&2
  echo "example: $0 v0.1.0" >&2
  exit 1
fi

VERSION_TAG="$1"
if [[ "${VERSION_TAG}" != v* ]]; then
  echo "version tag must start with 'v' (got: ${VERSION_TAG})" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
BUILD_DIR="${ROOT_DIR}/.tmp/release"

VERSION="${VERSION_TAG#v}"
COMMIT="${GIT_COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short=12 HEAD)}"
BUILT_AT="${BUILT_AT:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

LDFLAGS="-s -w -X github.com/daulet/k11s/internal/buildinfo.Version=${VERSION} -X github.com/daulet/k11s/internal/buildinfo.Commit=${COMMIT} -X github.com/daulet/k11s/internal/buildinfo.BuiltAt=${BUILT_AT}"

rm -rf "${DIST_DIR}" "${BUILD_DIR}"
mkdir -p "${DIST_DIR}" "${BUILD_DIR}"

build_one() {
  local goarch="$1"
  local archive_suffix="$2"
  local out_dir="${BUILD_DIR}/${archive_suffix}"

  mkdir -p "${out_dir}"

  echo "building darwin/${goarch}..."
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 GOOS=darwin GOARCH="${goarch}" \
      go build -trimpath -ldflags "${LDFLAGS}" -o "${out_dir}/k11s" ./cmd/k11s
    CGO_ENABLED=0 GOOS=darwin GOARCH="${goarch}" \
      go build -trimpath -ldflags "${LDFLAGS}" -o "${out_dir}/k11sd" ./cmd/k11sd
  )

  chmod +x "${out_dir}/k11s" "${out_dir}/k11sd"

  local tarball="${DIST_DIR}/k11s-${archive_suffix}.tar.gz"
  tar -C "${out_dir}" -czf "${tarball}" k11s k11sd
}

build_one amd64 x86_64-apple-darwin
build_one arm64 aarch64-apple-darwin

if command -v sha256sum >/dev/null 2>&1; then
  (
    cd "${DIST_DIR}"
    sha256sum k11s-*.tar.gz > checksums.txt
  )
else
  (
    cd "${DIST_DIR}"
    shasum -a 256 k11s-*.tar.gz > checksums.txt
  )
fi

echo
echo "release artifacts created:"
ls -1 "${DIST_DIR}"

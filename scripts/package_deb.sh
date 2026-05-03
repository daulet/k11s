#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Package k11s and k11sd as a Debian archive.

Usage:
  package_deb.sh --cli PATH --daemon PATH --version X.Y.Z --arch amd64 --out-dir dist --desc TEXT
EOF
}

CLI_PATH=""
DAEMON_PATH=""
VERSION=""
ARCH=""
OUT_DIR=""
DESC=""
PACKAGE_NAME="k11s"
HOMEPAGE="https://github.com/daulet/k11s"
MAINTAINER="Daulet <noreply@github.com>"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cli)
      CLI_PATH="$2"
      shift 2
      ;;
    --daemon)
      DAEMON_PATH="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    --arch)
      ARCH="$2"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --desc)
      DESC="$2"
      shift 2
      ;;
    --homepage)
      HOMEPAGE="$2"
      shift 2
      ;;
    --maintainer)
      MAINTAINER="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${CLI_PATH}" || -z "${DAEMON_PATH}" || -z "${VERSION}" || -z "${ARCH}" || -z "${OUT_DIR}" ]]; then
  usage >&2
  exit 2
fi

if [[ -z "${DESC}" ]]; then
  DESC="Speed-first CLI/TUI for Kubernetes navigation and operations"
fi

if [[ ! -f "${CLI_PATH}" ]]; then
  echo "cli binary not found: ${CLI_PATH}" >&2
  exit 1
fi
if [[ ! -f "${DAEMON_PATH}" ]]; then
  echo "daemon binary not found: ${DAEMON_PATH}" >&2
  exit 1
fi

case "${ARCH}" in
  amd64 | arm64)
    ;;
  *)
    echo "unsupported Debian architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

PKG_ROOT="${WORK_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}"
LIBEXEC_DIR="${PKG_ROOT}/usr/lib/${PACKAGE_NAME}"
mkdir -p "${PKG_ROOT}/DEBIAN" "${PKG_ROOT}/usr/bin" "${LIBEXEC_DIR}" "${PKG_ROOT}/usr/share/doc/${PACKAGE_NAME}"

install -m 755 "${CLI_PATH}" "${LIBEXEC_DIR}/k11s"
install -m 755 "${DAEMON_PATH}" "${LIBEXEC_DIR}/k11sd"
if [[ -f README.md ]]; then
  install -m 644 README.md "${PKG_ROOT}/usr/share/doc/${PACKAGE_NAME}/README.md"
fi

cat > "${PKG_ROOT}/usr/bin/k11s" <<'EOF'
#!/usr/bin/env sh
set -eu
export K11SD_PATH="${K11SD_PATH:-/usr/lib/k11s/k11sd}"
exec /usr/lib/k11s/k11s "$@"
EOF
chmod 755 "${PKG_ROOT}/usr/bin/k11s"

INSTALLED_SIZE="$(du -sk "${PKG_ROOT}/usr" | awk '{print $1}')"

cat > "${PKG_ROOT}/DEBIAN/control" <<EOF
Package: ${PACKAGE_NAME}
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: ${MAINTAINER}
Installed-Size: ${INSTALLED_SIZE}
Homepage: ${HOMEPAGE}
Description: ${DESC}
EOF

mkdir -p "${OUT_DIR}"
dpkg-deb --build --root-owner-group "${PKG_ROOT}" "${OUT_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"


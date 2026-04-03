#!/usr/bin/env bash
set -euo pipefail

REPO="nullne/multica"
BINARY="multica"
INSTALL_DIR="/usr/local/bin"

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       error "Unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)             error "Unsupported architecture: $(uname -m)" ;;
  esac
}

get_latest_version() {
  local url="https://api.github.com/repos/${REPO}/releases/latest"
  local json
  if command -v curl &>/dev/null; then
    json="$(curl -fsSL "$url")"
  elif command -v wget &>/dev/null; then
    json="$(wget -qO- "$url")"
  else
    error "curl or wget is required"
  fi
  printf '%s' "$json" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name" *: *"([^"]+)".*/\1/'
}

download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL -o "$dest" "$url"
  else
    wget -qO "$dest" "$url"
  fi
}

TMPDIR_CLEANUP=""
trap 'rm -rf "$TMPDIR_CLEANUP"' EXIT

main() {
  local os arch version archive_name download_url

  os="$(detect_os)"
  arch="$(detect_arch)"

  info "Detected platform: ${os}/${arch}"

  if [ -n "${MULTICA_VERSION:-}" ]; then
    version="$MULTICA_VERSION"
    info "Using specified version: ${version}"
  else
    info "Fetching latest release..."
    version="$(get_latest_version)"
  fi

  [ -z "$version" ] && error "Could not determine version. Set MULTICA_VERSION or check https://github.com/${REPO}/releases"

  if command -v "$BINARY" &>/dev/null; then
    local current
    current="$("$BINARY" version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1)" || true
    if [ "$current" = "$version" ]; then
      info "${BINARY} ${version} is already installed"
      exit 0
    fi
    if [ -n "$current" ]; then
      info "Upgrading ${BINARY} from ${current} to ${version}"
    fi
  fi

  archive_name="${BINARY}_${os}_${arch}.tar.gz"
  download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

  info "Downloading ${BINARY} ${version}..."
  TMPDIR_CLEANUP="$(mktemp -d)"
  local tmpdir="$TMPDIR_CLEANUP"

  download "$download_url" "${tmpdir}/${archive_name}"

  info "Extracting..."
  tar -xzf "${tmpdir}/${archive_name}" -C "$tmpdir"

  [ -f "${tmpdir}/${BINARY}" ] || error "Binary not found in archive"

  info "Installing to ${INSTALL_DIR}/${BINARY}..."
  if [ -w "$INSTALL_DIR" ]; then
    mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  else
    sudo mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  fi
  chmod +x "${INSTALL_DIR}/${BINARY}"

  info "Installed ${BINARY} ${version} to ${INSTALL_DIR}/${BINARY}"
  "${INSTALL_DIR}/${BINARY}" version 2>/dev/null || true
}

main

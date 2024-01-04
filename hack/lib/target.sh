#!/usr/bin/env bash

function seal::target::build_prefix() {
  local prefix
  prefix="$(basename "${ROOT_DIR}")"

  if [[ -n "${BUILD_PREFIX:-}" ]]; then
    echo -n "${BUILD_PREFIX}"
  else
    echo -n "${prefix}"
  fi
}

readonly DEFAULT_BUILD_TAGS=(
  "netgo"
  "jsoniter"
)

function seal::target::build_tags() {
  local target="${1:-}"

  local tags
  if [[ -n "${BUILD_TAGS:-}" ]]; then
    IFS="," read -r -a tags <<<"${BUILD_TAGS}"
  else
    case "${target}" in
    utils)
      tags=()
      ;;
    *)
      tags=("${DEFAULT_BUILD_TAGS[@]}")
      ;;
    esac
  fi

  if [[ ${#tags[@]} -ne 0 ]]; then
    echo -n "${tags[@]}"
  fi
}

readonly DEFAULT_BUILD_PLATFORMS=(
  linux/amd64
  linux/arm64
  darwin/amd64
  darwin/arm64
  windows/amd64
)

function seal::target::build_platforms() {
  local target="${1:-}"

  local platforms
  if [[ -z "${OS:-}" ]] && [[ -z "${ARCH:-}" ]]; then
    platforms=("${DEFAULT_BUILD_PLATFORMS[@]}")
  else
    local os="${OS:-$(seal::util::get_raw_os)}"
    local arch="${ARCH:-$(seal::util::get_raw_arch)}"
    platforms=("${os}/${arch}")
  fi

  if [[ ${#platforms[@]} -ne 0 ]]; then
    echo -n "${platforms[@]}"
  fi
}

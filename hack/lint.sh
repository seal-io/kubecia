#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "${ROOT_DIR}/hack/lib/init.sh"

function check_dirty() {
  [[ "${LINT_DIRTY:-false}" == "true" ]] || return 0

  if [[ -n "$(command -v git)" ]]; then
    if git_status=$(git status --porcelain 2>/dev/null) && [[ -n ${git_status} ]]; then
      seal::log::fatal "the git tree is dirty:\n$(git status --porcelain)"
    fi
  fi
}

function lint() {
  local target="$1"
  local path="$2"
  local path_ignored="$3"

  local build_tags=()
  read -r -a build_tags <<<"$(seal::target::build_tags "${target}")"

  [[ "${path}" == "${ROOT_DIR}" ]] || pushd "${path}" >/dev/null 2>&1

  seal::format::run "${path}" "${path_ignored}"
  if [[ ${#build_tags[@]} -gt 0 ]]; then
    GOLANGCI_LINT_CACHE="$(go env GOCACHE)/golangci-lint" seal::lint::run --build-tags="\"${build_tags[*]}\"" "${path}/..."
  else
    GOLANGCI_LINT_CACHE="$(go env GOCACHE)/golangci-lint" seal::lint::run "${path}/..."
  fi

  [[ "${path}" == "${ROOT_DIR}" ]] || popd >/dev/null 2>&1
}

function after() {
  check_dirty
}

#
# main
#

seal::log::info "+++ LINT +++"

seal::commit::lint "${ROOT_DIR}"

lint "kubecia" "${ROOT_DIR}" "" "$@"

after

seal::log::info "--- LINT ---"

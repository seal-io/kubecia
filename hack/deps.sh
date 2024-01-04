#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "${ROOT_DIR}/hack/lib/init.sh"

function mod() {
  local target="$1"
  local path="$2"

  seal::log::debug "modding ${target}"

  [[ "${path}" == "${ROOT_DIR}" ]] || pushd "${path}" >/dev/null 2>&1

  go mod tidy
  go mod download

  [[ "${path}" == "${ROOT_DIR}" ]] || popd >/dev/null 2>&1
}

#
# main
#

seal::log::info "+++ MOD +++"

mod "kubecia" "${ROOT_DIR}" "$@"

seal::log::info "--- MOD ---"

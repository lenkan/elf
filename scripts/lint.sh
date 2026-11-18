#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
files=("$root"/bin/* "$root"/scripts/*.sh "$root"/install.sh)

echo "==> shellcheck"
shellcheck -x "${files[@]}"

echo "==> shfmt"
shfmt --diff "${files[@]}"

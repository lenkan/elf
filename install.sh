#!/usr/bin/env bash
# Installs elf into ~/.local/bin, with bash completion. Run it again to upgrade:
#   curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
#
#   ELF_VERSION  release to install, like v0.2.0 (default: the latest)
#   BINDIR       where to put it (default: ~/.local/bin)
#
# Everything is inside main, so a download cut off halfway runs nothing.
set -euo pipefail

main() {
	local bindir="${BINDIR:-$HOME/.local/bin}" os arch binary url file
	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) die "no build for $(uname -s)" ;;
	esac
	case "$(uname -m)" in
	x86_64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "no build for $(uname -m)" ;;
	esac
	url=https://github.com/lenkan/elf/releases/latest/download
	if [ -n "${ELF_VERSION:-}" ]; then
		url=https://github.com/lenkan/elf/releases/download/$ELF_VERSION
	fi

	binary="elf-$os-$arch"
	echo "downloading elf ${ELF_VERSION:-(latest)} for $os/$arch"
	tmp="$(mktemp -d)" # global: the EXIT trap runs after main returns
	trap 'rm -rf "$tmp"' EXIT
	for file in "$binary" SHA256SUMS; do
		curl -fsSL -o "$tmp/$file" "$url/$file" || die "couldn't download $url/$file"
	done
	(cd "$tmp" && grep " $binary\$" SHA256SUMS | sha256 --check --status) || die "checksum mismatch"

	mkdir -p "$bindir"
	install -m 755 "$tmp/$binary" "$bindir/elf"
	echo "installed $bindir/elf ($("$bindir/elf" --version))"
	local compdir="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
	mkdir -p "$compdir"
	"$bindir/elf" completion bash >"$compdir/elf"

	case ":$PATH:" in
	*":$bindir:"*) ;;
	*) warn "$bindir is not on your PATH; add it in your shell's rc file" ;;
	esac
	command -v git >/dev/null || warn "elf needs git, not found"
	echo "next: elf build"
}

# sha256 is sha256sum on Linux, shasum on macOS.
sha256() {
	if command -v sha256sum >/dev/null; then
		sha256sum "$@"
	else
		shasum -a 256 "$@"
	fi
}

die() {
	echo "elf install: $*" >&2
	exit 1
}

warn() {
	echo "elf install: warning: $*" >&2
}

main "$@"

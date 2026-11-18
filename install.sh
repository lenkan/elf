#!/usr/bin/env bash
# Installs elf:
#   curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
#
# Downloads a release binary into ~/.local/bin. Run it again to upgrade.
#
#   ELF_VERSION  release to install, like v0.2.0 (default: the latest)
#   BINDIR       where to put it (default: ~/.local/bin)
#
# Everything is inside main, so a download cut off halfway runs nothing.
set -euo pipefail

main() {
	local bindir="${BINDIR:-$HOME/.local/bin}" arch url file
	[ "$(uname -s)" = Linux ] || die "elf runs on Linux only"
	case "$(uname -m)" in
	x86_64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "no build for $(uname -m)" ;;
	esac
	url=https://github.com/lenkan/elf/releases/latest/download
	if [ -n "${ELF_VERSION:-}" ]; then
		url=https://github.com/lenkan/elf/releases/download/$ELF_VERSION
	fi

	echo "downloading elf ${ELF_VERSION:-(latest)} for linux/$arch"
	tmp="$(mktemp -d)" # global: the EXIT trap runs after main returns
	trap 'rm -rf "$tmp"' EXIT
	for file in "elf-linux-$arch" SHA256SUMS; do
		curl -fsSL -o "$tmp/$file" "$url/$file" || die "couldn't download $url/$file"
	done
	(cd "$tmp" && sha256sum --check --ignore-missing --quiet SHA256SUMS) || die "checksum mismatch"

	mkdir -p "$bindir"
	install -m 755 "$tmp/elf-linux-$arch" "$bindir/elf"
	echo "installed $bindir/elf ($("$bindir/elf" version))"

	case ":$PATH:" in
	*":$bindir:"*) ;;
	*) warn "$bindir is not on your PATH; add it in your shell's rc file" ;;
	esac
	local cmd missing=()
	for cmd in git podman tmux; do
		command -v "$cmd" >/dev/null || missing+=("$cmd")
	done
	[ ${#missing[@]} -eq 0 ] || warn "elf needs ${missing[*]}, not found"
	echo "next: elf build && elf login"
}

die() {
	echo "elf install: $*" >&2
	exit 1
}

warn() {
	echo "elf install: warning: $*" >&2
}

main "$@"

#!/usr/bin/env bash
# Installs elf:
#   curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
#
# Downloads a release into ~/.local/share/elf and links bin/elf into
# ~/.local/bin. The whole release, not just the script: `elf build` needs
# the Dockerfile next to it. Run it again to upgrade.
#
# Run from a checkout (./install.sh), it links that checkout's bin/elf
# instead, so `git pull` updates the command.
#
#   ELF_VERSION  tag or branch to install (default: the latest release)
#   BINDIR       where to put the command (default: ~/.local/bin)
#
# Everything is inside main, so a download cut off halfway runs nothing.
set -euo pipefail

main() {
	local bindir="${BINDIR:-$HOME/.local/bin}"
	local share="${XDG_DATA_HOME:-$HOME/.local/share}/elf"
	local self="${BASH_SOURCE[0]:-}" root target

	if [ -e "$bindir/elf" ] && [ ! -L "$bindir/elf" ]; then
		die "$bindir/elf exists and isn't a link; remove it first"
	fi
	if [ -f "$self" ] && root="$(dirname "$(readlink -f "$self")")" && [ -f "$root/bin/elf" ]; then
		target="$root/bin/elf"
	else
		download "$share"
		target="$share/bin/elf"
	fi
	mkdir -p "$bindir"
	ln -sfn "$target" "$bindir/elf"
	echo "installed $bindir/elf -> $target"

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

# Fetches GitHub's source archive of a release into $share, replacing any
# earlier install.
download() {
	local share="$1" repo=lenkan/elf ref url tmp
	command -v curl >/dev/null || die "needs curl"

	ref="${ELF_VERSION:-}"
	if [ -z "$ref" ]; then
		# github.com/<repo>/releases/latest redirects to .../releases/tag/<tag>;
		# no API call, so no rate limit and no JSON to parse
		url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")" ||
			die "couldn't look up the latest release of github.com/$repo"
		[[ $url == */releases/tag/* ]] ||
			die "$repo has no releases yet; set ELF_VERSION to a tag or branch"
		ref="${url##*/releases/tag/}"
	fi

	echo "downloading elf $ref"
	mkdir -p "$(dirname "$share")"
	# Next to $share, so the final mv doesn't cross filesystems
	tmp="$(mktemp -d "$share.XXXXXX")"
	trap 'rm -rf "$tmp"' EXIT
	curl -fsSL "https://github.com/$repo/archive/$ref.tar.gz" |
		tar -xz -C "$tmp" --strip-components=1 ||
		die "couldn't download $ref from github.com/$repo"
	[ -f "$tmp/bin/elf" ] || die "$ref has no bin/elf"

	if [ -e "$share" ]; then
		[ -f "$share/bin/elf" ] || die "$share exists and isn't an elf install; move it away"
		rm -rf "$share"
	fi
	mv "$tmp" "$share"
	trap - EXIT
}

main "$@"

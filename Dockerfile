# The image elf agents run in. Add toolchains here.
FROM docker.io/library/node:24-bookworm

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ripgrep fd-find jq less \
	&& rm -rf /var/lib/apt/lists/*

# `elf build` passes the host's version, so the agent matches what you run
ARG OPENCODE_VERSION=2.3.2
ARG TARGETARCH
RUN case "$TARGETARCH" in \
		amd64) arch=x64 ;; \
		arm64) arch=arm64 ;; \
		*) echo "unsupported arch: $TARGETARCH" >&2 && exit 1 ;; \
	esac \
	&& curl -fsSL "https://github.com/anomalyco/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-${arch}.tar.gz" \
	| tar -xz -C /usr/local/bin opencode

# The agent runs as `node` (uid 1000); podman maps your host user onto it.
# Pre-create the mount points: podman would create them owned by root.
USER node
RUN mkdir -p /home/node/.config/opencode /home/node/.local/share/opencode

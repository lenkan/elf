# The image elf agents run in: elf build
FROM docker.io/library/ubuntu:26.04

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		build-essential ca-certificates curl fd-find git jq less libssl-dev \
		openssh-client pkg-config python3 python3-pip python3-venv ripgrep \
		unzip wget xz-utils zip \
	&& rm -rf /var/lib/apt/lists/* \
	&& ln -s /usr/bin/fdfind /usr/local/bin/fd

ARG TARGETARCH
RUN case "$TARGETARCH" in \
		amd64 | arm64) ;; \
		*) echo "unsupported arch: $TARGETARCH" >&2 && exit 1 ;; \
	esac

ARG GO_VERSION=1.27.4
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" | tar -xz -C /usr/local

# Match your opencode: --build-arg OPENCODE_VERSION=<your version>
ARG OPENCODE_VERSION=2.3.2
RUN arch=$([ "$TARGETARCH" = amd64 ] && echo x64 || echo arm64) \
	&& curl -fsSL "https://github.com/anomalyco/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-${arch}.tar.gz" \
	| tar -xz -C /usr/local/bin opencode

RUN userdel --remove ubuntu 2>/dev/null; useradd --create-home --uid 1000 --shell /bin/bash agent
USER agent
WORKDIR /home/agent
# Not in a shell profile: the harness's shells don't read one
ENV PATH=/home/agent/.local/bin:/home/agent/.cargo/bin:/home/agent/go/bin:/usr/local/go/bin:/home/agent/.local/share/fnm/aliases/default/bin:$PATH

RUN curl --proto '=https' -fsSL https://sh.rustup.rs \
	| sh -s -- -y --no-modify-path --profile minimal --default-toolchain stable --component clippy,rustfmt

ARG FNM_VERSION=1.39.0
ARG NODE_VERSION=26
RUN zip=$([ "$TARGETARCH" = amd64 ] && echo fnm-linux || echo fnm-arm64) \
	&& mkdir -p ~/.local/bin \
	&& curl -fsSL -o /tmp/fnm.zip "https://github.com/Schniz/fnm/releases/download/v${FNM_VERSION}/${zip}.zip" \
	&& unzip -q /tmp/fnm.zip -d ~/.local/bin && chmod +x ~/.local/bin/fnm && rm /tmp/fnm.zip \
	&& fnm install "$NODE_VERSION" && fnm default "$NODE_VERSION"
# In a repo with .nvmrc or .node-version, bash switches to its Node
ENV SHELL=/bin/bash BASH_ENV=/home/agent/.bash_env
RUN printf '%s\n' \
	'eval "$(fnm env --shell bash)"' \
	'if [ -f .nvmrc ] || [ -f .node-version ]; then fnm use --install-if-missing --silent-if-unchanged >/dev/null 2>&1; fi' \
	> ~/.bash_env

ARG UV_VERSION=0.13.9
RUN curl -fsSL "https://astral.sh/uv/${UV_VERSION}/install.sh" | env UV_NO_MODIFY_PATH=1 sh

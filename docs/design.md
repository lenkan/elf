# Design direction

The current version is a prototype: opencode only. This is where it's headed.

## Principles

- **You and the CLI set up the boundary; the agent only runs inside it.** The whole harness runs in the container. An agent never decides whether to hand off, or with what access. Agent integrations, if any, wrap the CLI and never replace it.
- **The container is the source of truth.** tmux is an optional integration.
- **The review loop is the product.** You check out the agent's branch in your own checkout, add commits and hand it back. Isolation alone is becoming a built-in harness feature.
- **Harness-neutral.** opencode, Claude Code, Codex and Copilot all get the same boundary and the same policy.

## Git boundary (kept from the prototype)

- The agent writes only its own clone. Your machine runs nothing but `git fetch` against it, because its config and hooks are the agent's.
- Your commits reach the agent through a bare repo that only your machine writes to, mounted read-only.

## Planned

- **Per-repo toolchain.** The repo commits `.elf/Dockerfile`, which describes only the project's toolchain (`FROM rust:…`). elf builds its own layer on top: the harness, a home directory, mount points. The image is tagged by a hash of `.elf/` and rebuilt when that changes. Without the file, a default image is used.
- **Harnesses as data.** Each harness is a definition: how to install it, start it with a prompt in auto mode, resume it, its credential volume, its API hosts, and an optional status hook. There are built-in definitions plus user config, because the CLI flags change often.
- **Status.** Harness hooks write `working` / `needs you` / `done`, and `elf ls` shows it.
- **Network policy.** Containers have no network except a host proxy. It allowlists hosts: model APIs, package registries, GitHub.
- **GitHub policy proxy.** The token stays on the host. The container's `gh` talks to a unix socket (`http_unix_socket`). Policy is per repo and read-only by default; GraphQL is POST, so queries are parsed and mutations rejected.
- **A config file** next to the Dockerfile, added once a second setting is needed. The likely first is a setup command such as `npm ci` after the clone.

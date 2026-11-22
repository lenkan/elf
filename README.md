# elf

Runs opencode agents in podman containers, each on its own branch. You review the branch in your checkout, then hand it back.

## Setup

Needs podman.

```bash
curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
elf build   # the agent's image
elf login   # model provider login, once
```

Rerun the installer to upgrade. To add toolchains to the image, copy the [Dockerfile](Dockerfile) and `elf build -f yours`. From a clone: `make install`.

The installer sets up bash completion (it needs the bash-completion package). For other shells, see `elf completion --help`.

To uninstall, `elf rm` your tasks, then:

```bash
rm -rf ~/.local/bin/elf ~/.local/share/bash-completion/completions/elf ~/.elf
podman volume rm elf-opencode   # your login
podman rmi localhost/elf
```

## Usage

From your repo:

```bash
elf start fix-login    # prompt opens in your git editor; -a to attach to it
elf ls
elf attach fix-login   # Ctrl+] to detach; the agent keeps running
elf review fix-login   # check out elf/fix-login here; again for its new commits
elf resume fix-login   # back to your branch; attach to give feedback
elf rm fix-login
```

Agents run in the background, so you can attach from anywhere, e.g. a tmux window: `tmux new-window elf attach fix-login`.

## What the agent can reach

- Its own clone, read-write, and a read-only copy of your repo.
- Its own credentials (`elf-opencode` volume) and a read-only copy of your opencode config.
- The network.

It runs with `--auto`, so it never asks for permission. Your machine only ever runs `git fetch` against its clone, so its hooks and config never run on your machine. Code you check out with `review` is still the agent's code: read it before you run it.

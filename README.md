# elf

Runs opencode agents in podman containers, each on its own branch. You review the branch in your checkout, then hand it back.

## Setup

Needs podman and tmux.

```bash
curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
elf build   # image from Dockerfile; add toolchains there
elf login   # model provider login, once
```

Rerun the installer to upgrade. From a clone, `./install.sh` links that clone instead.

To uninstall, `elf rm` your tasks, then:

```bash
rm -rf ~/.local/bin/elf ~/.local/share/elf ~/.elf
podman volume rm elf-opencode   # your login
podman rmi localhost/elf
```

## Usage

Inside tmux, from your repo:

```bash
elf start fix-login    # prompt opens in your git editor; -a to switch to it
elf ls
elf attach fix-login
elf review fix-login   # check out elf/fix-login here; again for its new commits
elf resume fix-login   # back to your branch; give feedback in its window
elf rm fix-login
```

tmux flags a window once its agent has been quiet for 30 seconds.

## What the agent can reach

- Its own clone, read-write, and a read-only copy of your repo.
- Its own credentials (`elf-opencode` volume) and a read-only copy of your opencode config.
- The network.

It runs with `--auto`, so it never asks for permission. Your machine only ever runs `git fetch` against its clone, so its hooks and config never run on your machine. Code you check out with `review` is still the agent's code: read it before you run it.

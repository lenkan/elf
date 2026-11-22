# elf

Runs coding agents in containers, so they can't read or run anything on your machine. You chat with them from your terminal and review their work with git.

## Setup

Needs opencode, git, and docker or podman. elf uses docker if it's installed; `ELF_ENGINE=podman` picks podman.

```bash
curl -fsSL https://raw.githubusercontent.com/lenkan/elf/main/install.sh | bash
```

Or `make install` from a clone. Then build the agent's image, with your opencode's version:

```bash
elf build
```

It has common tools, plus fnm, rustup, Go and uv for the versions your repo asks for. `elf --image` uses another image: it needs opencode, git and a non-root user.

## Usage

Run `elf` in your repo:

```
tasks:
  1  elf/brave-otter          running  2 new   Fix the login redirect

task number, or enter for a new one: 1
elf/brave-otter: attach, review or delete? [a/r/d, enter attaches]
```

- **New task:** an agent in its own container, on a copy of your current commit. Tell it what to do, then quit the chat with Ctrl+C; it keeps working.
- **Attach:** back to its chat.
- **Review:** checks out its branch here, with its new commits. Run it, tweak it, commit. The next `elf` from that branch hands your commits to the agent.
- **Delete:** removes the container, and keeps the agent's work in your branch.

## What the agent can reach

- Its own repo.
- A copy of your opencode config, and your model logins, which it can read.
- The network.

Nothing of your machine is mounted, and its git runs in its container. Code you check out is still the agent's: read it before you run it.

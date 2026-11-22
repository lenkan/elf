# Design

## Goal

Hand coding tasks to agents that can't reach your machine, and stay in the loop: review, run, tweak and give feedback in your own checkout.

## Principles

- **The container is the boundary.** Rules inside the agent's harness can be talked around; a container can't. Nothing of your machine is mounted.
- **The harness runs inside it.** Only the chat reaches you.
- **Review is plain git**, through a remote whose git runs in the container.
- **One container per task**, so agents can run dev servers without clashing.

## Open

- **Credentials.** The agent can read your model logins.
- **Network.** The agent can send out anything it can read; an allowlisting proxy would limit that.
- **GitHub access** through a proxy that keeps the token on your machine.
- **Remote engines** and **other harnesses**.

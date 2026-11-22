package main

// detachKeys detaches you from an agent. Podman's default, Ctrl+P Ctrl+Q,
// conflicts with the harnesses' TUIs.
const detachKeys = "ctrl-]"

// containers runs agents, each in its own podman container.
type containers interface {
	imageExists() bool
	// state returns the task's container state, like "running" or "exited",
	// or "" if it has no container.
	state(t *task) string
	// start runs opencode in the task's container, in the background.
	start(t *task, opencodeArgs ...string) error
	// attach connects your terminal to the agent until you detach.
	attach(t *task) error
	remove(t *task) error
}

type podmanContainers struct{}

func (podmanContainers) imageExists() bool {
	return succeeds("podman", "image", "exists", image())
}

func (podmanContainers) state(t *task) string {
	state, err := output("podman", "container", "inspect", "--format", "{{.State.Status}}", t.container)
	if err != nil {
		return ""
	}
	return state
}

func (podmanContainers) start(t *task, opencodeArgs ...string) error {
	// No --rm: an agent that exits keeps its output in podman logs
	args := []string{"run", "--detach", "--interactive", "--tty"}
	args = append(args, podmanRun()...)
	args = append(args,
		"--name", t.container,
		"-v", configDir()+":/home/node/.config/opencode:ro,z",
		"-v", t.hostGit+":"+t.hostGit+":ro,z",
		"-v", t.work+":"+t.work+":z",
		"-w", t.work,
		image(), "opencode", "--standalone", "--auto", t.work)
	args = append(args, opencodeArgs...)
	_, err := output("podman", args...)
	return err
}

func (podmanContainers) attach(t *task) error {
	// No signal proxy: closing your terminal must not kill the agent
	return run("podman", "attach", "--detach-keys", detachKeys, "--sig-proxy=false", t.container)
}

func (podmanContainers) remove(t *task) error {
	_, err := output("podman", "rm", "--force", "--ignore", t.container)
	return err
}

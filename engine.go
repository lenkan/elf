package main

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	//go:embed Dockerfile
	dockerfile []byte
	//go:embed container/serve.sh
	serveScript string
	//go:embed container/setup.sh
	setupScript string
	//go:embed container/chat.sh
	chatScript string
	//go:embed container/title.sh
	titleScript string
	//go:embed container/instructions.md
	instructions string
)

type engine struct {
	cli   string
	image string
}

func connect(image string) (engine, error) {
	cli := os.Getenv("ELF_ENGINE")
	if cli != "" {
		return engine{cli: cli, image: image}, nil
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return engine{cli: "docker", image: image}, nil
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return engine{cli: "podman", image: image}, nil
	}
	return engine{}, errors.New("elf needs docker or podman")
}

// create creates the task's container. services are the names of its compose
// services, which share its network: they resolve to localhost.
func (e engine) create(t *task, services []string) error {
	credentials, err := output("opencode", "auth", "export")
	if err != nil {
		return err
	}
	name, _ := git("config", "user.name")
	email, _ := git("config", "user.email")

	args := []string{"create",
		"--pull", "never",
		"--init",
		"--name", t.container,
		"--label", "elf.repo=" + t.repo,
		"--label", "elf.task=" + t.name,
		"--label", "elf.branch=" + t.branch,
		"--hostname", "elf",
		"--env", "ELF_INSTRUCTIONS=" + instructions,
		"--env", "ELF_CREDENTIALS=" + credentials,
		"--env", "OPENCODE_PASSWORD=" + rand.Text(),
		"--env", "GIT_AUTHOR_NAME=" + name,
		"--env", "GIT_AUTHOR_EMAIL=" + email,
		"--env", "GIT_COMMITTER_NAME=" + name,
		"--env", "GIT_COMMITTER_EMAIL=" + email,
	}
	for _, service := range services {
		args = append(args, "--add-host", service+":127.0.0.1")
	}
	args = append(args, e.image, "sh", "-c", serveScript)
	_, err = output(e.cli, args...)
	if err != nil {
		return fmt.Errorf("%w\nno image %s? build it: elf build", err, e.image)
	}

	err = e.setUp(t)
	if err != nil {
		e.remove(t)
		return err
	}
	return nil
}

func (e engine) setUp(t *task) error {
	_, err := output(e.cli, "start", t.container)
	if err != nil {
		return err
	}
	_, err = output(e.cli, "exec", t.container, "sh", "-c", setupScript, "sh", t.branch)
	if err != nil {
		return err
	}
	home, err := output(e.cli, "exec", t.container, "printenv", "HOME")
	if err != nil {
		return err
	}
	t.work = home + "/work/repo"
	err = e.copyConfig(t, home+"/.config/opencode")
	if err != nil {
		return err
	}
	return e.copyModel(t)
}

// copyModel gives the agent the model you last picked in opencode, which it
// keeps in its state, not its config.
func (e engine) copyModel(t *task) error {
	state := filepath.Join(os.Getenv("HOME"), ".local", "state", "opencode")
	if os.Getenv("XDG_STATE_HOME") != "" {
		state = filepath.Join(os.Getenv("XDG_STATE_HOME"), "opencode")
	}
	model, err := os.ReadFile(filepath.Join(state, "model.json"))
	if err != nil {
		return nil
	}
	// Written by the container's user, so opencode can save your next pick
	cmd := exec.Command(e.cli, "exec", "--interactive", t.container, "sh", "-c",
		"mkdir -p ~/.local/state/opencode && cat > ~/.local/state/opencode/model.json")
	cmd.Stdin = bytes.NewReader(model)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s exec: %s", e.cli, strings.TrimSpace(string(out)))
	}
	return nil
}

// copyConfig follows symlinks (tar -h): they wouldn't resolve in the container.
func (e engine) copyConfig(t *task, dir string) error {
	yours := filepath.Join(os.Getenv("HOME"), ".config", "opencode")
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		yours = filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "opencode")
	}
	var items []string
	for _, item := range []string{"opencode.json", "opencode.jsonc", "AGENTS.md", "agents", "commands", "skills"} {
		_, err := os.Stat(filepath.Join(yours, item))
		if err == nil {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}

	tarArgs := append([]string{"-chf", "-", "-C", yours}, items...)
	archive, err := exec.Command("tar", tarArgs...).Output()
	if err != nil {
		return fmt.Errorf("tar: %w", err)
	}
	cp := exec.Command(e.cli, "cp", "-", t.container+":"+dir)
	cp.Stdin = bytes.NewReader(archive)
	out, err := cp.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s cp: %s", e.cli, strings.TrimSpace(string(out)))
	}
	return nil
}

// build builds elf's Dockerfile, with the version of your opencode.
func (e engine) build() error {
	dir, err := os.MkdirTemp("", "elf-build")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	err = os.WriteFile(filepath.Join(dir, "Dockerfile"), dockerfile, 0o644)
	if err != nil {
		return err
	}

	out, err := output("opencode", "--version")
	if err != nil {
		return err
	}
	// "opencode v2.3.2"
	fields := strings.Fields(out)
	version := strings.TrimPrefix(fields[len(fields)-1], "v")
	return run(e.cli, "build",
		"--tag", e.image,
		"--build-arg", "OPENCODE_VERSION="+version,
		dir,
	)
}

func (e engine) remove(t *task) error {
	_, err := output(e.cli, "compose", "--project-name", t.container, "down", "--volumes")
	if err != nil {
		return err
	}
	_, err = output(e.cli, "rm", "--force", t.container)
	return err
}

func (e engine) services(file string) ([]string, error) {
	out, err := output(e.cli, "compose", "--file", file, "config", "--services")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// startServices starts the task's own copy of the compose services, in the
// network of its container.
func (e engine) startServices(t *task, file string, services []string) error {
	override := "services:\n"
	for _, service := range services {
		override += "  " + service + ":\n"
		override += "    network_mode: container:" + t.container + "\n"
		// Published ports and fixed names would clash between tasks
		override += "    ports: !reset []\n"
		override += "    container_name: !reset null\n"
		override += "    networks: !reset []\n"
	}
	overrideFile, err := os.CreateTemp("", "elf-compose-*.yml")
	if err != nil {
		return err
	}
	defer os.Remove(overrideFile.Name())
	_, err = overrideFile.WriteString(override)
	if err != nil {
		return err
	}
	overrideFile.Close()

	return run(e.cli, "compose",
		"--project-name", t.container,
		"--file", file,
		"--file", overrideFile.Name(),
		"up", "--detach",
	)
}

func (e engine) tasks(repo string) ([]string, error) {
	out, err := output(e.cli, "ps",
		"--all",
		"--filter", "label=elf.repo="+repo,
		"--filter", "label=elf.task",
		"--format", "{{.Names}}",
	)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, container := range strings.Fields(out) {
		name := strings.TrimPrefix(container, "elf-")
		names = append(names, name)
	}
	return names, nil
}

func (e engine) state(t *task) string {
	state, _ := output(e.cli, "container", "inspect", "--format", "{{.State.Status}}", t.container)
	return state
}

func (e engine) title(t *task) string {
	if e.state(t) != "running" {
		return ""
	}
	out, err := output(e.cli, "exec", t.container, "sh", "-c", titleScript, "sh", t.work)
	if err != nil {
		return ""
	}
	var sessions []struct{ Title string }
	json.Unmarshal([]byte(out), &sessions)
	if len(sessions) == 0 {
		return ""
	}
	return sessions[0].Title
}

func (e engine) ensureRunning(t *task) error {
	if e.state(t) == "running" {
		return nil
	}
	_, err := output(e.cli, "start", t.container)
	if err != nil {
		return err
	}
	// The services lost the network of the stopped container
	_, err = output(e.cli, "compose", "--project-name", t.container, "restart")
	return err
}

func (e engine) chat(t *task) error {
	err := e.ensureRunning(t)
	if err != nil {
		return err
	}
	return run(e.cli, "exec", "--interactive", "--tty", t.container, "sh", "-c", chatScript, "sh", t.work)
}

// gitURL runs git's side of fetch and push in the container, so your machine
// never reads the agent's repo.
func (e engine) gitURL(t *task) string {
	return "ext::" + e.cli + " exec -i " + t.container + " %S " + t.work
}

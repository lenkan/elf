package main

import (
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const volume = "elf-opencode"

var (
	//go:embed Dockerfile
	dockerfile []byte
	//go:embed prompt.md
	promptTemplate string
)

func build(file string) error {
	dir := filepath.Dir(file)
	if file == "" {
		tmp, err := os.MkdirTemp("", "elf-build")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		dir, file = tmp, filepath.Join(tmp, "Dockerfile")
		if err := os.WriteFile(file, dockerfile, 0o644); err != nil {
			return err
		}
	}
	podman := []string{"build", "-t", image(), "-f", file}
	// Match the agent's opencode to yours
	if out, err := exec.Command("opencode", "--version").Output(); err == nil {
		if v := regexp.MustCompile(`\d+\.\d+\.\d+`).FindString(string(out)); v != "" {
			podman = append(podman, "--build-arg", "OPENCODE_VERSION="+v)
		}
	}
	return run("podman", append(podman, dir)...)
}

func login() error {
	args := []string{"run", "--rm", "--interactive", "--tty"}
	args = append(args, podmanRun()...)
	args = append(args, image(), "opencode", "--standalone", "auth", "login")
	return run("podman", args...)
}

// start starts an agent on a new task. An empty prompt opens your editor.
func start(c containers, name, base, prompt string, switchTo bool) error {
	t, err := newTask(name)
	if err != nil {
		return err
	}

	if !c.imageExists() {
		return fmt.Errorf("no image %s, run: elf build", image())
	}
	if _, err := os.Stat(t.dir); err == nil {
		return fmt.Errorf("task %q already exists", t.name)
	}
	if t.local() != "" {
		return fmt.Errorf("branch %s already exists", t.branch)
	}
	sha, err := git("rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil {
		return fmt.Errorf("unknown base %q", base)
	}
	baseName := base
	if baseName == "HEAD" {
		baseName = cmp.Or(currentBranch(), sha)
	}

	if prompt == "" {
		if prompt, err = readPrompt(t, baseName); err != nil {
			return err
		}
	}
	if strings.TrimSpace(prompt) == "" {
		return errors.New("empty prompt, nothing started")
	}

	if err := t.create(sha); err != nil {
		return err
	}
	if err := refreshConfig(); err != nil {
		return err
	}
	full := strings.NewReplacer("{branch}", t.branch, "{base}", baseName, "{prompt}", strings.TrimSpace(prompt)).
		Replace(promptTemplate)
	if err := c.start(t, "--prompt", full); err != nil {
		return err
	}
	fmt.Printf("started %s on %s from %s. Watch it: elf attach %s\n", t.name, t.branch, baseName, t.name)
	if out, _ := git("status", "--porcelain"); out != "" {
		fmt.Println("note: the agent doesn't get your uncommitted changes")
	}
	if switchTo {
		return attach(c, t)
	}
	return nil
}

const scissors = "# ------------------------ >8 ------------------------"

// readPrompt reads the prompt from stdin, or from your git editor the way
// `git commit` does.
func readPrompt(t *task, baseName string) (string, error) {
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	dir, err := os.MkdirTemp("", "elf-prompt")
	if err != nil {
		return "", err
	}
	file := filepath.Join(dir, "PROMPT.md")
	template := fmt.Sprintf(`
%s
# Task %s, on %s from %s.
# The agent sees only this prompt: give it the context, the constraints and
# how to verify (tests, lint, build). Everything from the line above down is
# ignored. Save an empty prompt to abort.
`, scissors, t.name, t.branch, baseName)
	if err := os.WriteFile(file, []byte(template), 0o644); err != nil {
		return "", err
	}
	editor, err := git("var", "GIT_EDITOR")
	if err != nil {
		return "", err
	}
	// GIT_EDITOR may hold arguments, so git runs it through sh too
	if err := run("sh", "-c", editor+` "$@"`, editor, file); err != nil {
		return "", fmt.Errorf("editor failed, prompt left in %s", file)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var prompt strings.Builder
	for line := range strings.Lines(string(b)) {
		if strings.TrimRight(line, "\n") == scissors {
			break
		}
		prompt.WriteString(line)
	}
	return prompt.String(), os.RemoveAll(dir)
}

// taskNames lists this repo's tasks.
func taskNames() ([]string, error) {
	_, repo, err := repoInfo()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(elfHome(), repo))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && taskName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func ls(c containers) error {
	names, err := taskNames()
	if err != nil {
		return err
	}
	cur := currentBranch()
	for _, name := range names {
		t, err := newTask(name)
		if err != nil {
			return err
		}
		state := "stopped"
		if cur == t.branch {
			state = "reviewing"
		} else if s := c.state(t); s != "" {
			state = s
		}
		fmt.Printf("%-30s %s\n", t.name, state)
	}
	return nil
}

func attach(c containers, t *task) error {
	state := c.state(t)
	if state != "running" {
		if state != "" {
			fmt.Printf("%s had exited, restarting it\n", t.name)
			if err := c.remove(t); err != nil {
				return err
			}
		}
		if err := refreshConfig(); err != nil {
			return err
		}
		if err := c.start(t, "--continue"); err != nil {
			return err
		}
	}
	fmt.Printf("attaching to %s, detach with %s\n", t.name, detachKeys)
	return c.attach(t)
}

func rm(c containers, t *task, force bool) error {
	if currentBranch() == t.branch {
		return fmt.Errorf("you're on %s; switch away first", t.branch)
	}
	if !force {
		theirs, err := t.fetch()
		if err != nil {
			return err
		}
		reviewed := t.local()
		if reviewed == "" {
			if reviewed, err = t.read("base"); err != nil {
				return err
			}
		}
		if !isAncestor(theirs, reviewed) {
			return fmt.Errorf("the agent has commits you haven't reviewed (elf review %s), or use rm -f", t.name)
		}
	}
	if err := c.remove(t); err != nil {
		return err
	}
	if err := os.RemoveAll(t.dir); err != nil {
		return fmt.Errorf("couldn't delete everything; try: podman unshare rm -rf %s", t.dir)
	}
	fmt.Println("removed", t.name)
	if t.local() != "" {
		fmt.Printf("your %s is kept: git branch -D %[1]s\n", t.branch)
	}
	return nil
}

// podmanRun returns the podman run arguments every agent container shares.
func podmanRun() []string {
	name, _ := git("config", "user.name")
	email, _ := git("config", "user.email")
	return []string{"--hostname", "elf",
		"--userns=keep-id:uid=1000,gid=1000",
		"-e", "GIT_AUTHOR_NAME=" + name, "-e", "GIT_AUTHOR_EMAIL=" + email,
		"-e", "GIT_COMMITTER_NAME=" + name, "-e", "GIT_COMMITTER_EMAIL=" + email,
		"-v", volume + ":/home/node/.local/share/opencode:U"}
}

// configDir is where elf keeps its copy of your opencode config.
func configDir() string {
	return filepath.Join(elfHome(), "config")
}

// refreshConfig copies (not mounts) your opencode config: symlinks out of it
// wouldn't resolve in the container, and :z would relabel the files they
// point to.
func refreshConfig() error {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	src, dst := filepath.Join(userConfig, "opencode"), configDir()
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, item := range []string{"cli.json", "opencode.json", "agents", "commands", "skills"} {
		if _, err := os.Stat(filepath.Join(src, item)); err != nil {
			continue
		}
		if _, err := output("cp", "-rL", filepath.Join(src, item), dst+"/"); err != nil {
			return err
		}
	}
	return nil
}

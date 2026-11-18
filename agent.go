package main

import (
	"cmp"
	_ "embed"
	"errors"
	"flag"
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

func build(args []string) error {
	flags := flag.NewFlagSet("build", flag.ExitOnError)
	file := flags.String("f", "", "build from your own `Dockerfile` instead of elf's")
	flags.Parse(args)

	dir := filepath.Dir(*file)
	if *file == "" {
		tmp, err := os.MkdirTemp("", "elf-build")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		dir, *file = tmp, filepath.Join(tmp, "Dockerfile")
		if err := os.WriteFile(*file, dockerfile, 0o644); err != nil {
			return err
		}
	}
	podman := []string{"build", "-t", image(), "-f", *file}
	// Match the agent's opencode to yours
	if out, err := exec.Command("opencode", "--version").Output(); err == nil {
		if v := regexp.MustCompile(`\d+\.\d+\.\d+`).FindString(string(out)); v != "" {
			podman = append(podman, "--build-arg", "OPENCODE_VERSION="+v)
		}
	}
	return run("podman", append(podman, dir)...)
}

func login() error {
	return run("podman", append(podmanRun(), image(), "opencode", "--standalone", "auth", "login")...)
}

func start(args []string) error {
	flags := flag.NewFlagSet("start", flag.ExitOnError)
	switchTo := flags.Bool("a", false, "switch to the agent's window")
	base := flags.String("b", "HEAD", "start from `commit`")
	flags.Parse(args)
	if flags.NArg() == 0 {
		return errUsage
	}
	t, err := newTask(flags.Arg(0))
	if err != nil {
		return err
	}

	if err := requireTmux(); err != nil {
		return err
	}
	if !succeeds("podman", "image", "exists", image()) {
		return fmt.Errorf("no image %s, run: elf build", image())
	}
	if _, err := os.Stat(t.dir); err == nil {
		return fmt.Errorf("task %q already exists", t.name)
	}
	if t.local() != "" {
		return fmt.Errorf("branch %s already exists", t.branch)
	}
	sha, err := git("rev-parse", "--verify", "--quiet", *base+"^{commit}")
	if err != nil {
		return fmt.Errorf("unknown base %q", *base)
	}
	baseName := *base
	if baseName == "HEAD" {
		baseName = cmp.Or(currentBranch(), sha)
	}

	prompt := strings.Join(flags.Args()[1:], " ")
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
	if _, err := runAgent(t, "--prompt", full); err != nil {
		return err
	}
	fmt.Printf("started %s on %s from %s. Watch it: elf attach %s\n", t.name, t.branch, baseName, t.name)
	if out, _ := git("status", "--porcelain"); out != "" {
		fmt.Println("note: the agent doesn't get your uncommitted changes")
	}
	if *switchTo {
		return attach(t)
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

func ls() error {
	_, repo, err := repoInfo()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(elfHome(), repo))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	cur := currentBranch()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := newTask(e.Name())
		if err != nil {
			continue
		}
		state := "stopped"
		if cur == t.branch {
			state = "reviewing"
		} else if succeeds("podman", "container", "exists", t.container) {
			state = "running"
		}
		fmt.Printf("%-30s %s\n", t.name, state)
	}
	return nil
}

func attach(t *task) error {
	if err := requireTmux(); err != nil {
		return err
	}
	id := findWindow(t)
	// remain-on-exit kept a failed agent's window; replace it
	if id != "" {
		if dead, _ := output("tmux", "display-message", "-p", "-t", id, "#{pane_dead}"); dead == "1" {
			if _, err := output("tmux", "kill-window", "-t", id); err != nil {
				return err
			}
			id = ""
		}
	}
	if id == "" {
		if succeeds("podman", "container", "exists", t.container) {
			return fmt.Errorf("%s is running without a tmux window; podman stop %[1]s", t.container)
		}
		if err := refreshConfig(); err != nil {
			return err
		}
		var err error
		if id, err = runAgent(t, "--continue"); err != nil {
			return err
		}
	}
	if _, err := output("tmux", "select-window", "-t", id); err != nil {
		return err
	}
	session, err := output("tmux", "display-message", "-p", "-t", id, "#{session_id}")
	if err != nil {
		return err
	}
	_, err = output("tmux", "switch-client", "-t", session)
	return err
}

func rm(args []string) error {
	flags := flag.NewFlagSet("rm", flag.ExitOnError)
	force := flags.Bool("f", false, "remove even if the agent has commits you haven't reviewed")
	flags.Parse(args)
	t, err := existingTask(flags.Args())
	if err != nil {
		return err
	}
	if currentBranch() == t.branch {
		return fmt.Errorf("you're on %s; switch away first", t.branch)
	}
	if !*force {
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
	if id := findWindow(t); id != "" {
		if _, err := output("tmux", "kill-window", "-t", id); err != nil {
			return err
		}
	}
	if _, err := output("podman", "rm", "-f", "--ignore", t.container); err != nil {
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

// podmanRun returns the podman arguments every agent container shares.
func podmanRun() []string {
	name, _ := git("config", "user.name")
	email, _ := git("config", "user.email")
	return []string{"run", "--rm", "-it", "--hostname", "elf",
		"--userns=keep-id:uid=1000,gid=1000",
		"-e", "GIT_AUTHOR_NAME=" + name, "-e", "GIT_AUTHOR_EMAIL=" + email,
		"-e", "GIT_COMMITTER_NAME=" + name, "-e", "GIT_COMMITTER_EMAIL=" + email,
		"-v", volume + ":/home/node/.local/share/opencode:U"}
}

// runAgent opens a tmux window running opencode in the task's container, and
// returns the window's id.
func runAgent(t *task, opencodeArgs ...string) (string, error) {
	podman := append([]string{"podman"}, podmanRun()...)
	podman = append(podman, "--name", t.container,
		"-v", configDir()+":/home/node/.config/opencode:ro,z",
		"-v", t.hostGit+":"+t.hostGit+":ro,z",
		"-v", t.work+":"+t.work+":z",
		"-w", t.work,
		image(), "opencode", "--standalone", "--auto", t.work)
	podman = append(podman, opencodeArgs...)
	id, err := output("tmux", append([]string{"new-window", "-d", "-P", "-F", "#{window_id}", "-n", t.name, "--"}, podman...)...)
	if err != nil {
		return "", err
	}
	for _, opt := range [][]string{
		{"@elf", t.container},
		// Keep the window if podman or opencode fails, so the error stays readable
		{"remain-on-exit", "failed"},
		// Flags the window in the status bar once the agent goes quiet: done, or waiting
		{"monitor-silence", "30"},
	} {
		if _, err := output("tmux", "set-option", "-w", "-t", id, opt[0], opt[1]); err != nil {
			return "", err
		}
	}
	return id, nil
}

func findWindow(t *task) string {
	out, _ := output("tmux", "list-windows", "-a", "-F", "#{window_id} #{@elf}")
	for line := range strings.Lines(out) {
		if id, name, _ := strings.Cut(strings.TrimSpace(line), " "); name == t.container {
			return id
		}
	}
	return ""
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

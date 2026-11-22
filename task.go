package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	taskName   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	nameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
)

type task struct {
	name      string
	top       string // your checkout
	repo      string
	branch    string // elf/<name>, in your repo and in the agent's clone
	dir       string
	hostGit   string
	work      string
	container string
}

func newTask(name string) (*task, error) {
	if !taskName.MatchString(name) {
		return nil, fmt.Errorf("task name: lowercase letters, digits, . _ - (got %q)", name)
	}
	top, repo, err := repoInfo()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(elfHome(), repo, name)
	return &task{
		name:      name,
		top:       top,
		repo:      repo,
		branch:    "elf/" + name,
		dir:       dir,
		hostGit:   filepath.Join(dir, "host.git"),
		work:      filepath.Join(dir, "work"),
		container: "elf-" + nameUnsafe.ReplaceAllString(repo, "-") + "-" + name,
	}, nil
}

// existingTask returns the task with this name, which must exist.
func existingTask(name string) (*task, error) {
	t, err := newTask(name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(t.dir); err != nil {
		return nil, fmt.Errorf("no task %q in %s (elf ls)", t.name, t.repo)
	}
	return t, nil
}

// repoInfo returns your checkout's top directory, and the repo's name: the
// main checkout's directory, so all worktrees of a repo share their tasks.
func repoInfo() (top, repo string, err error) {
	top, err = git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", errors.New("not in a git repository")
	}
	common, err := git("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	return top, filepath.Base(filepath.Dir(common)), nil
}

// create sets up the task's repos: host.git for your commits, work/ for the agent's.
func (t *task) create(base string) error {
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{
		// --local hardlinks objects, so this is cheap. Safe: the container only mounts it read-only.
		{"clone", "--quiet", "--bare", "--local", t.top, t.hostGit},
		{"--git-dir=" + t.hostGit, "remote", "remove", "origin"},
		{"--git-dir=" + t.hostGit, "branch", t.branch, base},
		// --shared borrows host.git's objects instead of copying them; new commits land in work/
		{"clone", "--quiet", "--shared", "--branch", t.branch, t.hostGit, t.work},
	} {
		if _, err := git(args...); err != nil {
			return err
		}
	}
	return t.write("base", base)
}

// fetch gets the agent's branch into FETCH_HEAD and returns its commit. The
// only git command elf runs against the agent's clone.
func (t *task) fetch() (string, error) {
	if _, err := git("fetch", "--quiet", "--no-tags", t.work, t.branch); err != nil {
		return "", err
	}
	return git("rev-parse", "FETCH_HEAD")
}

// read returns the contents of one of the task's state files.
func (t *task) read(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(t.dir, name))
	return strings.TrimSpace(string(b)), err
}

func (t *task) write(name, value string) error {
	return os.WriteFile(filepath.Join(t.dir, name), []byte(value+"\n"), 0o644)
}

func elfHome() string {
	home, _ := os.UserHomeDir()
	return cmp.Or(os.Getenv("ELF_HOME"), filepath.Join(home, ".elf"))
}

func image() string {
	return cmp.Or(os.Getenv("ELF_IMAGE"), "localhost/elf")
}

func currentBranch() string {
	b, _ := git("symbolic-ref", "--short", "-q", "HEAD")
	return b
}

func requireClean() error {
	if out, err := git("status", "--porcelain", "--untracked-files=no"); err != nil || out != "" {
		return errors.New("commit or stash your changes first")
	}
	return nil
}

// local returns the commit of your elf/<task> branch, or "" if you have none.
func (t *task) local() string {
	sha, _ := git("rev-parse", "--verify", "--quiet", "refs/heads/"+t.branch)
	return sha
}

func isAncestor(a, b string) bool {
	return succeeds("git", "merge-base", "--is-ancestor", a, b)
}

// succeeds reports whether a command exits 0, ignoring its output.
func succeeds(name string, args ...string) bool {
	return exec.Command(name, args...).Run() == nil
}

func git(args ...string) (string, error) {
	return output("git", args...)
}

// output runs a command and returns its trimmed stdout. Its stderr goes into
// the error.
func output(name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := cmp.Or(strings.TrimSpace(stderr.String()), err.Error())
		return "", fmt.Errorf("%s %s: %s", name, args[0], msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// run runs a command on your terminal.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, args[0], err)
	}
	return nil
}

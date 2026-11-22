package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var nameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

type task struct {
	name      string
	repo      string
	branch    string
	remote    string
	theirs    string // your ref of the agent's branch
	container string
	work      string // the agent's repo, in its container
}

func newTask(repo, name string) *task {
	t := &task{
		name:      name,
		repo:      repo,
		branch:    "elf/" + name,
		remote:    "elf-" + name,
		theirs:    "refs/remotes/elf-" + name + "/elf/" + name,
		container: "elf-" + name,
	}
	// The remote's URL ends with the agent's repo; see gitURL
	url, err := git("remote", "get-url", t.remote)
	if err == nil {
		fields := strings.Fields(url)
		t.work = fields[len(fields)-1]
	}
	return t
}

// repoName is the main checkout's directory, so all worktrees share their tasks.
func repoName() (string, error) {
	common, err := git("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", errors.New("not in a git repository")
	}
	return nameUnsafe.ReplaceAllString(filepath.Base(filepath.Dir(common)), "-"), nil
}

func currentBranch() string {
	b, _ := git("symbolic-ref", "--short", "-q", "HEAD")
	return b
}

func git(args ...string) (string, error) {
	return output("git", args...)
}

// output returns the command's stdout; its stderr goes into the error.
func output(name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, args[0], msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, args[0], err)
	}
	return nil
}

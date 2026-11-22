package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeAgents stands in for the containers: each agent's repo is a directory,
// set up like the container does.
type fakeAgents struct {
	dir   string
	names []string // newest first
	chats []string
}

// repo is the agent's repo of task name.
func (f *fakeAgents) repo(name string) string {
	return filepath.Join(f.dir, name)
}

func (f *fakeAgents) tasks(repo string) ([]string, error) {
	return f.names, nil
}

func (f *fakeAgents) state(t *task) string {
	if slices.Contains(f.names, t.name) {
		return "running"
	}
	return ""
}

func (f *fakeAgents) title(t *task) string {
	return ""
}

func (f *fakeAgents) ensureRunning(t *task) error {
	return nil
}

func (f *fakeAgents) gitURL(t *task) string {
	return f.repo(t.name)
}

func (f *fakeAgents) chat(t *task) error {
	f.chats = append(f.chats, t.name)
	return nil
}

func (f *fakeAgents) remove(t *task) error {
	f.names = slices.DeleteFunc(f.names, func(n string) bool { return n == t.name })
	return os.RemoveAll(f.repo(t.name))
}

func (f *fakeAgents) create(t *task) error {
	dir := f.repo(t.name)
	for _, args := range [][]string{
		{"init", "--quiet", dir},
		{"-C", dir, "symbolic-ref", "HEAD", "refs/heads/" + t.branch},
		{"-C", dir, "config", "receive.denyCurrentBranch", "updateInstead"},
	} {
		if _, err := git(args...); err != nil {
			return err
		}
	}
	f.names = append([]string{t.name}, f.names...)
	return nil
}

// setup makes a repo with one commit and changes into it.
func setup(t *testing.T) *fakeAgents {
	tmp := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(k+"_NAME", "elf")
		t.Setenv(k+"_EMAIL", "elf@example.com")
	}
	repo := filepath.Join(tmp, "repo")
	mustGit(t, "init", "--quiet", "--initial-branch=main", repo)
	t.Chdir(repo)
	commit(t, "", "init")
	return &fakeAgents{dir: filepath.Join(tmp, "agents")}
}

func mustElf(t *testing.T, a *fakeAgents, input string) {
	t.Helper()
	if err := elf(a, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// commit adds a file named msg and commits it, in dir ("" for your checkout).
func commit(t *testing.T, dir, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, msg), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "-C", dir, "add", msg)
	mustGit(t, "-C", dir, "commit", "--quiet", "-m", msg)
}

func mustGit(t *testing.T, args ...string) string {
	t.Helper()
	out, err := git(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFirstRunStartsATaskOnYourCommit(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	if len(a.chats) != 1 || a.chats[0] != a.names[0] {
		t.Fatalf("chats = %q, want the new task %q", a.chats, a.names)
	}
	got := mustGit(t, "-C", a.repo(a.names[0]), "rev-parse", "HEAD")
	if want := mustGit(t, "rev-parse", "main"); got != want {
		t.Errorf("the agent's HEAD = %s, want your %s", got, want)
	}
}

func TestPickATaskByNumberOrStartANewOne(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	first := a.names[0]
	mustElf(t, a, "\n")
	second := a.names[0]
	mustElf(t, a, "2\n") // newest first, so 2 is the first task
	if want := []string{first, second, first}; !slices.Equal(a.chats, want) {
		t.Errorf("chats = %q, want %q", a.chats, want)
	}
	if err := elf(a, strings.NewReader("7\n")); err == nil {
		t.Error("picked a task that doesn't exist")
	}
}

func TestReviewChecksOutTheAgentsBranch(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	name := a.names[0]
	commit(t, a.repo(name), "agent")

	mustElf(t, a, "1\nr\n")
	if got := currentBranch(); got != "elf/"+name {
		t.Fatalf("on %s, want elf/%s", got, name)
	}
	if _, err := os.Stat("agent"); err != nil {
		t.Fatalf("the agent's commit isn't checked out: %v", err)
	}

	commit(t, a.repo(name), "more")
	mustElf(t, a, "1\nr\n")
	if _, err := os.Stat("more"); err != nil {
		t.Errorf("reviewing again didn't bring the agent's new commit: %v", err)
	}
}

func TestRunElfFromTheBranchToHandBack(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	name := a.names[0]
	mustElf(t, a, "1\nr\n")
	commit(t, "", "mine")

	mustElf(t, a, "1\n")
	if _, err := os.Stat(filepath.Join(a.repo(name), "mine")); err != nil {
		t.Errorf("your commit isn't in the agent's working tree: %v", err)
	}
}

func TestDeleteKeepsTheAgentsWork(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	name := a.names[0]
	commit(t, a.repo(name), "agent")

	mustElf(t, a, "1\nd\n")
	if got := mustGit(t, "log", "--format=%s", "-1", "elf/"+name); got != "agent" {
		t.Errorf("elf/%s is at %q, want the agent's commit", name, got)
	}
	if remotes := mustGit(t, "remote"); len(a.names) != 0 || remotes != "" {
		t.Errorf("the task is still there: %q, remotes %q", a.names, remotes)
	}
}

func TestDeleteAsksBeforeLosingWork(t *testing.T) {
	a := setup(t)
	mustElf(t, a, "")
	name := a.names[0]
	mustElf(t, a, "1\nr\n")
	commit(t, "", "mine") // never handed back
	mustGit(t, "switch", "--quiet", "main")
	commit(t, a.repo(name), "agent")

	mustElf(t, a, "1\nd\nn\n")
	if len(a.names) != 1 {
		t.Fatal("deleted without asking")
	}
	mustElf(t, a, "1\nd\ny\n")
	if len(a.names) != 0 {
		t.Error("not deleted after you said yes")
	}
}

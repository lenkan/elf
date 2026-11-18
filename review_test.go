package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setup makes a repo with one commit and a task "demo" on it, as `elf start`
// would minus the container, and changes into the repo.
func setup(t *testing.T) *task {
	tmp := t.TempDir()
	t.Setenv("ELF_HOME", filepath.Join(tmp, "elf"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(k+"_NAME", "elf")
		t.Setenv(k+"_EMAIL", "elf@example.com")
	}
	t.Setenv("TMUX", "")

	repo := filepath.Join(tmp, "repo")
	mustGit(t, "init", "--quiet", "--initial-branch=main", repo)
	t.Chdir(repo)
	commit(t, "", "init")
	tk, err := newTask("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.create(rev(t, "", "HEAD")); err != nil {
		t.Fatal(err)
	}
	return tk
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

// squash makes the agent squash its branch into one commit on the base.
func squash(t *testing.T, tk *task) {
	t.Helper()
	base, _ := tk.read("base")
	mustGit(t, "-C", tk.work, "reset", "--quiet", "--soft", base)
	mustGit(t, "-C", tk.work, "commit", "--quiet", "-m", "squashed")
}

func rev(t *testing.T, dir, ref string) string {
	t.Helper()
	return mustGit(t, "-C", dir, "rev-parse", ref)
}

func mustGit(t *testing.T, args ...string) string {
	t.Helper()
	out, err := git(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustReview(t *testing.T, tk *task) {
	t.Helper()
	if err := review(tk); err != nil {
		t.Fatal(err)
	}
}

func TestReviewFollowsAgent(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	mustReview(t, tk)
	if cur := currentBranch(); cur != tk.branch {
		t.Fatalf("on %s, want %s", cur, tk.branch)
	}

	mustReview(t, tk) // nothing new
	commit(t, tk.work, "a2")
	mustReview(t, tk)
	if got, want := rev(t, "", "HEAD"), rev(t, tk.work, "HEAD"); got != want {
		t.Errorf("HEAD = %s, want the agent's %s", got, want)
	}
}

func TestReviewKeepsYourCommitsAhead(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	mustReview(t, tk)
	commit(t, "", "m1")
	mine := rev(t, "", "HEAD")
	mustReview(t, tk)
	if got := rev(t, "", "HEAD"); got != mine {
		t.Errorf("HEAD moved to %s, want your %s", got, mine)
	}
}

func TestReviewTakesRewriteOfEverythingYouGave(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	mustReview(t, tk)
	commit(t, "", "m1")
	if err := resume(tk); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "-C", tk.work, "pull", "--quiet", "--ff-only")
	squash(t, tk)

	mustReview(t, tk)
	if got, want := rev(t, "", tk.branch), rev(t, tk.work, "HEAD"); got != want {
		t.Errorf("%s = %s, want the agent's squash %s", tk.branch, got, want)
	}
}

func TestReviewRefusesToLoseYourCommits(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	mustReview(t, tk)
	commit(t, "", "m1") // never resumed
	mine := rev(t, "", "HEAD")
	squash(t, tk)

	err := review(tk)
	if err == nil {
		t.Fatal("review took the agent's squash over your unsent commit")
	}
	msg := err.Error()
	if !strings.Contains(msg, " m1") || strings.Contains(msg, " a1") || !strings.Contains(msg, "cherry-pick") {
		t.Errorf("error should list only m1 and suggest a cherry-pick:\n%s", msg)
	}
	if got := rev(t, "", "HEAD"); got != mine {
		t.Errorf("HEAD moved to %s, want your %s", got, mine)
	}
}

func TestReviewAsksToResumeWhenBothMoved(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	mustReview(t, tk)
	commit(t, "", "m1")
	commit(t, tk.work, "a2")

	err := review(tk)
	if err == nil || !strings.Contains(err.Error(), "elf resume demo") {
		t.Fatalf("want an error suggesting elf resume, got %v", err)
	}
}

func TestReviewRefusesDirtyTree(t *testing.T) {
	tk := setup(t)
	if err := os.WriteFile("init", []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := review(tk); err == nil {
		t.Fatal("review ran with uncommitted changes")
	}
}

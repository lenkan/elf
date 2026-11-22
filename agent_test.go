package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// fakeContainers stands in for podman. Set its fields to describe the
// agent's state, then check calls for what elf did to it.
type fakeContainers struct {
	imageBuilt     bool
	containerState string // "" for no container
	calls          []string
}

func (f *fakeContainers) imageExists() bool {
	return f.imageBuilt
}

func (f *fakeContainers) state(t *task) string {
	return f.containerState
}

func (f *fakeContainers) start(t *task, opencodeArgs ...string) error {
	f.calls = append(f.calls, "start "+opencodeArgs[0])
	return nil
}

func (f *fakeContainers) attach(t *task) error {
	f.calls = append(f.calls, "attach")
	return nil
}

func (f *fakeContainers) remove(t *task) error {
	f.calls = append(f.calls, "remove")
	return nil
}

func wantCalls(t *testing.T, c *fakeContainers, want ...string) {
	t.Helper()
	if !slices.Equal(c.calls, want) {
		t.Errorf("calls = %q, want %q", c.calls, want)
	}
}

func TestRmRemovesContainerAndClone(t *testing.T) {
	tk := setup(t)
	c := &fakeContainers{containerState: "running"}

	if err := rm(c, tk, false); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, c, "remove")
	if _, err := os.Stat(tk.dir); !os.IsNotExist(err) {
		t.Errorf("%s still exists", tk.dir)
	}
}

func TestRmRefusesUnreviewedCommits(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	c := &fakeContainers{containerState: "running"}

	err := rm(c, tk, false)
	if err == nil || !strings.Contains(err.Error(), "elf review demo") {
		t.Fatalf("want an error suggesting elf review, got %v", err)
	}
	wantCalls(t, c)
	if _, err := os.Stat(tk.dir); err != nil {
		t.Errorf("task deleted: %v", err)
	}
}

func TestRmForceIgnoresUnreviewedCommits(t *testing.T) {
	tk := setup(t)
	commit(t, tk.work, "a1")
	c := &fakeContainers{containerState: "running"}

	if err := rm(c, tk, true); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, c, "remove")
}

func TestAttachToRunningAgent(t *testing.T) {
	tk := setup(t)
	c := &fakeContainers{containerState: "running"}

	if err := attach(c, tk); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, c, "attach")
}

func TestAttachRestartsExitedAgent(t *testing.T) {
	tk := setup(t)
	c := &fakeContainers{containerState: "exited"}

	if err := attach(c, tk); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, c, "remove", "start --continue", "attach")
}

func TestAttachStartsAgentWithoutContainer(t *testing.T) {
	tk := setup(t)
	c := &fakeContainers{}

	if err := attach(c, tk); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, c, "start --continue", "attach")
}

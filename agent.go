package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	petname "github.com/dustinkirkland/golang-petname"
)

type agents interface {
	tasks(repo string) ([]string, error) // newest first
	state(t *task) string
	title(t *task) string
	create(t *task, services []string) error
	services(file string) ([]string, error)
	startServices(t *task, file string, services []string) error
	ensureRunning(t *task) error
	chat(t *task) error
	remove(t *task) error
	gitURL(t *task) string
}

func elf(a agents, input io.Reader) error {
	in := bufio.NewReader(input)
	repo, err := repoName()
	if err != nil {
		return err
	}
	names, err := a.tasks(repo)
	if err != nil {
		return err
	}

	name, onAgentBranch := strings.CutPrefix(currentBranch(), "elf/")
	if onAgentBranch && slices.Contains(names, name) {
		handBack(a, newTask(repo, name))
	}

	t, err := pickTask(a, repo, names, in)
	if err != nil {
		return err
	}
	if t == nil {
		t, err = startTask(a, repo)
		if err != nil {
			return err
		}
		return attach(a, t)
	}

	fmt.Printf("%s: attach, review or delete? [a/r/d, enter attaches] ", t.branch)
	switch ask(in) {
	case "", "a":
		return attach(a, t)
	case "r":
		return review(a, t)
	case "d":
		return remove(a, t, in)
	}
	return errors.New("pick a, r or d")
}

// ask reads a line of your answer.
func ask(in *bufio.Reader) string {
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}

func attach(a agents, t *task) error {
	err := a.chat(t)
	if err != nil {
		return err
	}
	return report(t)
}

// pickTask returns nil for a new task.
func pickTask(a agents, repo string, names []string, in *bufio.Reader) (*task, error) {
	if len(names) == 0 {
		return nil, nil
	}
	fmt.Println("tasks:")
	for i, name := range names {
		t := newTask(repo, name)
		state := a.state(t)
		if state == "running" {
			git("fetch", "--quiet", t.remote)
		}
		fmt.Printf("  %d  %-24s %-8s %-7s %s\n", i+1, t.branch, state, newCommits(t), a.title(t))
	}
	fmt.Print("task number, or enter for a new one: ")

	answer := ask(in)
	if answer == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(answer)
	if err != nil || n < 1 || n > len(names) {
		return nil, fmt.Errorf("no task %q", answer)
	}
	return newTask(repo, names[n-1]), nil
}

func startTask(a agents, repo string) (*task, error) {
	sha, err := git("rev-parse", "--verify", "HEAD")
	if err != nil {
		return nil, err
	}
	t := newTask(repo, freeName(a, repo))

	file := composeFile()
	var services []string
	if file != "" {
		services, err = a.services(file)
		if err != nil {
			return nil, err
		}
	}

	err = a.create(t, services)
	if err != nil {
		return nil, err
	}
	err = addRemote(a, t, sha)
	if err == nil && file != "" {
		fmt.Println("starting the services in", filepath.Base(file))
		err = a.startServices(t, file, services)
	}
	if err != nil {
		a.remove(t)
		git("remote", "remove", t.remote)
		return nil, err
	}

	status, _ := git("status", "--porcelain")
	if status != "" {
		fmt.Println("note: the agent doesn't get your uncommitted changes")
	}
	return t, nil
}

// composeFile returns your repo's compose file, preferring one just for elf, or
// "" if it has none.
func composeFile() string {
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	for _, name := range []string{"docker-compose.elf.yml", "compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		file := filepath.Join(top, name)
		_, err := os.Stat(file)
		if err == nil {
			return file
		}
	}
	return ""
}

func addRemote(a agents, t *task, sha string) error {
	// The remote runs git in the container, through the ext transport
	_, err := git("config", "protocol.ext.allow", "user")
	if err != nil {
		return err
	}
	_, err = git("remote", "add", t.remote, a.gitURL(t))
	if err != nil {
		return err
	}
	_, err = git("push", "--quiet", t.remote, sha+":refs/heads/"+t.branch)
	return err
}

func freeName(a agents, repo string) string {
	for {
		name := petname.Generate(2, "-")
		if !taken(a, newTask(repo, name)) {
			return name
		}
	}
}

func taken(a agents, t *task) bool {
	if a.state(t) != "" {
		return true
	}
	_, err := git("remote", "get-url", t.remote)
	if err == nil {
		return true
	}
	_, err = git("rev-parse", "--verify", "--quiet", "refs/heads/"+t.branch)
	return err == nil
}

func handBack(a agents, t *task) {
	err := a.ensureRunning(t)
	if err != nil {
		fmt.Println("elf:", err)
		return
	}
	_, err = git("fetch", "--quiet", t.remote)
	if err != nil {
		fmt.Println("elf:", err)
		return
	}
	yours, _ := git("rev-list", "--count", t.theirs+".."+t.branch)
	if yours == "0" {
		return
	}

	_, err = git("push", "--quiet", t.remote, t.branch)
	if err != nil {
		fmt.Printf("the agent didn't take your commits on %s; it has new commits or uncommitted work:\n", t.branch)
		fmt.Println(err)
		return
	}
	fmt.Println("gave the agent your commits on", t.branch)
}

// newCommits counts the agent's commits that none of your branches have.
func newCommits(t *task) string {
	count, err := git("rev-list", "--count", t.theirs, "--not", "--branches")
	if err != nil || count == "0" {
		return ""
	}
	return count + " new"
}

func report(t *task) error {
	_, err := git("fetch", "--quiet", t.remote)
	if err != nil {
		return err
	}
	commits, _ := git("log", "--oneline", t.theirs, "--not", "--branches")
	if commits == "" {
		return nil
	}
	fmt.Println()
	fmt.Println("new on " + t.branch + ":")
	fmt.Println(commits)
	fmt.Println()
	fmt.Println("review it: run elf, pick the task, then r")
	return nil
}

// review checks out the agent's branch here, with its new commits.
func review(a agents, t *task) error {
	err := a.ensureRunning(t)
	if err != nil {
		return err
	}
	_, err = git("fetch", "--quiet", t.remote)
	if err != nil {
		return err
	}
	commits, _ := git("log", "--oneline", t.theirs, "--not", "--branches")

	if currentBranch() != t.branch {
		// Creates elf/<task> the first time, tracking the agent's branch
		_, err = git("switch", "--quiet", t.branch)
		if err != nil {
			return err
		}
	}
	_, err = git("merge", "--quiet", "--ff-only", t.theirs)
	if err != nil {
		return fmt.Errorf("you and the agent both have new commits on %s; put yours on top: git rebase %s", t.branch, t.theirs)
	}

	if commits != "" {
		fmt.Println("new from the agent:")
		fmt.Println(commits)
		fmt.Println()
	}
	fmt.Println("on " + t.branch + ". Commit your changes, then run elf to hand them to the agent")
	return nil
}

// remove deletes the task, keeping the agent's work in your elf/<task>.
func remove(a agents, t *task, in *bufio.Reader) error {
	if currentBranch() == t.branch {
		return fmt.Errorf("you're on %s; switch away first", t.branch)
	}
	err := keepWork(a, t)
	if err != nil {
		fmt.Println(err)
		fmt.Print("delete it anyway, losing what the agent did? [y/N] ")
		if ask(in) != "y" {
			return nil
		}
	}

	err = a.remove(t)
	if err != nil {
		return err
	}
	git("remote", "remove", t.remote)
	fmt.Println("deleted", t.name)
	return nil
}

// keepWork fetches the agent's branch into your elf/<task>. It fails rather
// than drop commits of yours.
func keepWork(a agents, t *task) error {
	err := a.ensureRunning(t)
	if err != nil {
		return err
	}
	_, err = git("fetch", "--quiet", t.remote, t.branch+":"+t.branch)
	if err != nil {
		return fmt.Errorf("couldn't keep the agent's work as your %s:\n%w", t.branch, err)
	}
	return nil
}

// Command elf hands a task to an opencode agent in a podman container. You
// review its branch in your own checkout, then hand it back.
//
// Per task, ~/.elf/<repo>/<task>/ holds:
//
//	host.git  bare repo only the host writes; mounted read-only, the agent pulls from it
//	work/     the agent's clone, the only thing it can write
//	base      the commit the task started from
//	seen      the agent's branch at your last review
//	return    the branch to go back to after a review
//
// The host never runs git inside work/ except `git fetch`: its config and
// hooks are the agent's, and most git commands would honour them.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
)

const usage = `Usage: elf <command> [args]

  build [-f <dockerfile>]   build the container image
  login                     log in to a model provider inside the container (once)
  start [-a] [-b <base>] <task> [prompt...]
                            start an agent on branch elf/<task>, from <base>
                            (default HEAD). Without a prompt, opens your editor
                            (or reads stdin). -a switches to its window
  ls                        list this repo's tasks
  attach <task>             switch to the task's tmux window, restarting the agent if needed
  review <task>             fetch the agent's commits and check out elf/<task> here;
                            on it already, update to the agent's new commits
  resume <task>             give your commits to the agent and switch back to your branch
  rm [-f] <task>            stop the agent and delete its clone (keeps your local branch)
  version                   print elf's version
`

// version is set by release builds: -ldflags "-X main.version=v1.2.3".
var version string

var errUsage = errors.New("usage")

func main() {
	err := dispatch(os.Args[1:])
	if errors.Is(err, errUsage) {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "elf:", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "build":
		return build(args)
	case "login":
		return login()
	case "start":
		return start(args)
	case "ls":
		return ls()
	case "rm":
		return rm(args)
	case "attach":
		return withTask(args, attach)
	case "review":
		return withTask(args, review)
	case "resume":
		return withTask(args, resume)
	case "version", "--version":
		fmt.Println(versionString())
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return errUsage
}

func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "unknown"
}

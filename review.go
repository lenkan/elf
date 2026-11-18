package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func review(t *task) error {
	cur := currentBranch()
	if cur == "" {
		return errors.New("you're on a detached HEAD; switch to a branch first")
	}
	if other, ok := strings.CutPrefix(cur, "elf/"); ok && cur != t.branch {
		return fmt.Errorf("finish the other review first: elf resume %s", other)
	}
	if err := requireClean(); err != nil {
		return err
	}

	mine := t.local()
	theirs, err := t.fetch()
	if err != nil {
		return err
	}
	target := theirs
	switch {
	case mine == "" || isAncestor(mine, theirs):
	case isAncestor(theirs, mine):
		target = mine // you're ahead of the agent
	default:
		if err := t.checkRewrite(mine, theirs); err != nil {
			return err
		}
	}
	if err := t.write("seen", theirs); err != nil {
		return err
	}

	if cur == t.branch {
		// git won't fetch into the checked-out branch, so move it here
		if _, err := git("reset", "--quiet", "--keep", target); err != nil {
			return err
		}
		if target == mine {
			fmt.Println("no new commits from the agent")
			return nil
		}
		from, err := git("merge-base", mine, target)
		if err != nil {
			return err
		}
		return run("git", "--no-pager", "log", "--oneline", "--reverse", from+"..HEAD")
	}

	if _, err := git("branch", "--force", t.branch, target); err != nil {
		return err
	}
	if err := t.write("return", cur); err != nil {
		return err
	}
	if _, err := git("switch", "--quiet", t.branch); err != nil {
		return err
	}
	base, err := t.read("base")
	if err != nil {
		return err
	}
	if err := run("git", "--no-pager", "log", "--oneline", "--reverse", base+"..HEAD"); err != nil {
		return err
	}
	fmt.Printf("on %s. Commit any changes, then: elf resume %s\n", t.branch, t.name)
	return nil
}

// checkRewrite handles your branch (mine) and the agent's (theirs) both having
// moved on. That's fine if the agent rewrote its branch (you asked it to
// squash, say) and already had all your commits: its version wins. It fails
// if taking it would lose commits of yours.
func (t *task) checkRewrite(mine, theirs string) error {
	// The agent has what you last gave it, and what it had at your last review
	given, err := git("--git-dir="+t.hostGit, "rev-parse", t.branch)
	if err != nil {
		return err
	}
	seen, err := t.read("seen")
	if err != nil {
		seen, err = t.read("base")
		if err != nil {
			return err
		}
	}
	unknown := []string{mine, "^" + given, "^" + seen}
	if out, err := git(append([]string{"rev-list", "-1"}, unknown...)...); err != nil {
		return err
	} else if out == "" {
		fmt.Printf("the agent rewrote %s; taking its version\n", t.branch)
		return nil
	}

	commits, err := git(append([]string{"log", "--oneline", "--reverse"}, unknown...)...)
	if err != nil {
		return err
	}
	var msg strings.Builder
	msg.WriteString("you have commits the agent never got:\n")
	for line := range strings.Lines(commits + "\n") {
		msg.WriteString("  " + line)
	}
	if isAncestor(seen, theirs) {
		msg.WriteString("It has new commits too. Give it yours, then review again:\n")
		fmt.Fprintf(&msg, "  git switch %s && elf resume %s", t.branch, t.name)
	} else {
		msg.WriteString("And it rewrote its branch. Put them on top of its version, then review again:\n")
		fmt.Fprintf(&msg, "  git switch %s && git reset --keep %s && git cherry-pick %s",
			t.branch, theirs, strings.Join(unknown, " "))
	}
	return errors.New(msg.String())
}

func resume(t *task) error {
	if currentBranch() != t.branch {
		return fmt.Errorf("not on %s", t.branch)
	}
	back, err := t.read("return")
	if err != nil {
		return errors.New("no branch to return to; switch yourself")
	}
	if err := requireClean(); err != nil {
		return err
	}
	// Forced: after the agent rewrites its branch, yours no longer contains what
	// you gave it last time. host.git is only a drop box, so nothing is lost.
	if _, err := git("push", "--quiet", t.hostGit, "+"+t.branch+":"+t.branch); err != nil {
		return err
	}
	if _, err := git("switch", "--quiet", back); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(t.dir, "return")); err != nil {
		return err
	}
	fmt.Printf("back on %s. Tell the agent your feedback; it pulls your commits first\n", back)
	if os.Getenv("TMUX") != "" {
		return attach(t)
	}
	return nil
}

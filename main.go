package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var version string

func main() {
	err := rootCmd().Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "elf:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "elf",
		Short:         "Hand a task to an opencode agent in a podman container",
		Version:       versionString(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	c := podmanContainers{}

	var file string
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build the container image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return build(file)
		},
	}
	buildCmd.Flags().StringVarP(
		&file,
		"file",
		"f",
		"",
		"build from your own `Dockerfile` instead of elf's",
	)

	root.AddCommand(buildCmd)

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a model provider inside the container (once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return login()
		},
	}
	root.AddCommand(loginCmd)

	var base string
	var switchTo bool
	startCmd := &cobra.Command{
		Use:               "start <task> [prompt...]",
		Short:             "Start an agent on branch elf/<task>",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			prompt := strings.Join(args[1:], " ")
			return start(c, name, base, prompt, switchTo)
		},
	}
	startCmd.Flags().BoolVarP(
		&switchTo,
		"attach",
		"a",
		false,
		"attach to the agent once it has started",
	)
	startCmd.Flags().StringVarP(
		&base,
		"base",
		"b",
		"HEAD",
		"start from `commit`",
	)
	startCmd.RegisterFlagCompletionFunc("base", completeRefs)
	root.AddCommand(startCmd)

	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List this repo's tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ls(c)
		},
	}
	root.AddCommand(lsCmd)

	attachCmd := &cobra.Command{
		Use:               "attach <task>",
		Short:             "Attach to the agent, restarting it if it has exited",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTasks,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := existingTask(args[0])
			if err != nil {
				return err
			}
			return attach(c, t)
		},
	}
	root.AddCommand(attachCmd)

	reviewCmd := &cobra.Command{
		Use:               "review <task>",
		Short:             "Check out elf/<task> here, with the agent's new commits",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTasks,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := existingTask(args[0])
			if err != nil {
				return err
			}
			return review(t)
		},
	}
	root.AddCommand(reviewCmd)

	resumeCmd := &cobra.Command{
		Use:               "resume <task>",
		Short:             "Give your commits to the agent and switch back to your branch",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTasks,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := existingTask(args[0])
			if err != nil {
				return err
			}
			return resume(t)
		},
	}
	root.AddCommand(resumeCmd)

	var force bool
	rmCmd := &cobra.Command{
		Use:               "rm <task>",
		Short:             "Stop the agent and delete its clone (keeps your local branch)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTasks,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := existingTask(args[0])
			if err != nil {
				return err
			}
			return rm(c, t, force)
		},
	}
	rmCmd.Flags().BoolVarP(
		&force,
		"force",
		"f",
		false,
		"remove even if the agent has commits you haven't reviewed",
	)
	root.AddCommand(rmCmd)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print elf's version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(versionString())
		},
	}
	root.AddCommand(versionCmd)

	return root
}

func completeTasks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	tasks, _ := taskNames()
	return tasks, cobra.ShellCompDirectiveNoFileComp
}

func completeRefs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out, _ := git("for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes", "refs/tags")
	return strings.Fields(out), cobra.ShellCompDirectiveNoFileComp
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

package main

import (
	"fmt"
	"os"
	"runtime/debug"

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
	var image string
	root := &cobra.Command{
		Use:           "elf",
		Short:         "Chat with coding agents in containers",
		Version:       versionString(),
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := connect(image)
			if err != nil {
				return err
			}
			return elf(e, os.Stdin)
		},
	}
	root.PersistentFlags().StringVar(
		&image,
		"image",
		"elf",
		"the agents' `image`",
	)

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build the agent's image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := connect(image)
			if err != nil {
				return err
			}
			return e.build()
		},
	}
	root.AddCommand(buildCmd)

	psCmd := &cobra.Command{
		Use:   "ps",
		Short: "List the repo's tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := connect(image)
			if err != nil {
				return err
			}
			return ps(e, cmd.OutOrStdout())
		},
	}
	root.AddCommand(psCmd)

	return root
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

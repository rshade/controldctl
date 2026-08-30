package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}
	return root
}

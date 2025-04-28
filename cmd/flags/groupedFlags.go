// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type GroupedFlags struct {
	Name            string
	ShowFlag        string
	FlagSet         *pflag.FlagSet
	IsAlwaysVisible bool
}

// WithGroupedHelp returns a cobra-compatible help function that displays extra flag groups.
func WithGroupedHelp(groups []GroupedFlags) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, _ []string) {
		shownGroups := make(map[string]bool)

		// Handle group visibility decision (but do NOT unhide flags globally!)
		for _, group := range groups {
			if group.IsAlwaysVisible || flagExists(group.ShowFlag, os.Args) {
				shownGroups[group.Name] = true
			}
		}

		// Show normal usage help
		if err := cmd.Root().UsageFunc()(cmd); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error showing command usage: %v\n", err)
		}

		// Print each group section
		for _, group := range groups {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", group.Name)
			if shownGroups[group.Name] {
				group.FlagSet.VisitAll(func(flag *pflag.Flag) {
					// Even if the flag is "hidden", we manually print it here
					fmt.Fprintf(cmd.OutOrStdout(), "  --%s", flag.Name)
					if flag.Value.Type() != "bool" {
						fmt.Fprintf(cmd.OutOrStdout(), " %s", flag.Value.Type())
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\t%s\n", flag.Usage)
				})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  (hidden) Use %s to show these options\n", group.ShowFlag)
			}
		}
	}
}

func flagExists(target string, args []string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func RegisterFlagGroup(cmd *cobra.Command, groupName string, showFlag string, isAlwaysVisible bool, defineFlags func(set *pflag.FlagSet)) GroupedFlags {
	// Define the showFlag for controlling visibility
	show := false
	cmd.Flags().BoolVar(&show, showFlag, false, fmt.Sprintf("Show %s", groupName))

	// Always hide the showFlag itself
	cmd.Flags().Lookup(showFlag).Hidden = true

	// Create a new FlagSet for this group
	flagSet := pflag.NewFlagSet(groupName, pflag.ContinueOnError)

	// Caller defines the flags
	defineFlags(flagSet)

	// Add the flagSet to the cmd's global flag list
	cmd.Flags().AddFlagSet(flagSet)

	// Always hide the grouped flags globally
	flagSet.VisitAll(func(f *pflag.Flag) {
		cmd.Flags().Lookup(f.Name).Hidden = true
	})

	// Return GroupedFlags
	return GroupedFlags{
		Name:            groupName,
		ShowFlag:        "--" + showFlag,
		FlagSet:         flagSet,
		IsAlwaysVisible: isAlwaysVisible,
	}
}

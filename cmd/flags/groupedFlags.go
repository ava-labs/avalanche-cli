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
	UnhideFunc      func(cmd *cobra.Command) // Optional unhide hook
	IsAlwaysVisible bool
}

// WithGroupedHelp returns a cobra-compatible help function that displays extra flag groups.
func WithGroupedHelp(groups []GroupedFlags) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, _ []string) {
		shownGroups := make(map[string]bool)

		// Handle unhide funcs and mark groups that should be shown
		for _, group := range groups {
			if group.IsAlwaysVisible || flagExists(group.ShowFlag, os.Args) {
				shownGroups[group.Name] = true
				if group.UnhideFunc != nil {
					group.UnhideFunc(cmd)
				}
			}
		}

		// Show normal usage help
		if err := cmd.Root().UsageFunc()(cmd); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error showing command usage: %v\n", err)
		}

		// Print each group section
		for _, group := range groups {
			if shownGroups[group.Name] {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", group.Name)
				group.FlagSet.VisitAll(func(flag *pflag.Flag) {
					fmt.Fprintf(cmd.OutOrStdout(), "  --%s", flag.Name)
					if flag.Value.Type() != "bool" {
						fmt.Fprintf(cmd.OutOrStdout(), " %s", flag.Value.Type())
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\t%s\n", flag.Usage)
				})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n  (hidden) Use %s to show these options\n", group.Name, group.ShowFlag)
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

	// If the group is always visible, don't hide the showFlag
	cmd.Flags().Lookup(showFlag).Hidden = true

	// Create a new FlagSet for the group
	flagSet := pflag.NewFlagSet(groupName, pflag.ContinueOnError)

	// Let the caller define their flags in this flag set
	defineFlags(flagSet)

	// Add the flagSet to the cmd
	cmd.Flags().AddFlagSet(flagSet)

	flagSet.VisitAll(func(f *pflag.Flag) {
		cmd.Flags().Lookup(f.Name).Hidden = true
	})

	// Return the GroupedFlags struct
	return GroupedFlags{
		Name:            groupName,
		ShowFlag:        "--" + showFlag,
		FlagSet:         flagSet,
		IsAlwaysVisible: isAlwaysVisible,
		UnhideFunc: func(cmd *cobra.Command) {
			if !isAlwaysVisible {
				// Show the flags when the user explicitly calls the UnhideFunc
				flagSet.VisitAll(func(f *pflag.Flag) {
					cmd.Flags().Lookup(f.Name).Hidden = false
				})
			}
		},
	}
}

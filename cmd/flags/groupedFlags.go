// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type GroupedFlags struct {
	Name            string
	ShowFlag        string
	FlagSet         *pflag.FlagSet
	IsAlwaysVisible bool
}

// WithGroupedHelp returns a cobra Run function that shows help organized by groups.
// It first shows normal command usage, then prints grouped flags, hiding some unless explicitly requested.
func WithGroupedHelp(groups []GroupedFlags) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, _ []string) {
		shownGroups := determineShownGroups(groups)

		printUsage(cmd)

		for _, group := range groups {
			printGroup(cmd, group, shownGroups[group.Name])
		}
	}
}

// determineShownGroups decides which flag groups should be visible based on user input and group settings.
func determineShownGroups(groups []GroupedFlags) map[string]bool {
	shown := make(map[string]bool)
	for _, group := range groups {
		if group.IsAlwaysVisible || flagExists(group.ShowFlag, os.Args) {
			shown[group.Name] = true
		}
	}
	return shown
}

// printUsage prints the general command usage/help text.
func printUsage(cmd *cobra.Command) {
	if err := cmd.Root().UsageFunc()(cmd); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error showing command usage: %v\n", err)
	}
}

// printGroup prints a specific group of flags, properly formatted and optionally hidden if not shown.
func printGroup(cmd *cobra.Command, group GroupedFlags, isShown bool) {
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", group.Name)

	if !isShown {
		fmt.Fprintf(cmd.OutOrStdout(), "  (hidden) Use %s to show these options\n", group.ShowFlag)
		return
	}

	flags, maxLen := collectFlags(group.FlagSet)

	for _, flag := range flags {
		padding := strings.Repeat(" ", maxLen-len(flag.nameAndType)+2)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s%s%s\n", flag.nameAndType, padding, flag.usage)
	}
}

// flagInfo holds the formatted flag name/type and its usage string.
type flagInfo struct {
	nameAndType string
	usage       string
}

// collectFlags gathers all flags in a flag set, calculating the maximum width needed for alignment.
func collectFlags(flagSet *pflag.FlagSet) ([]flagInfo, int) {
	var flags []flagInfo
	maxLen := 0

	flagSet.VisitAll(func(flag *pflag.Flag) {
		nameAndType := "--" + flag.Name
		if flag.Value.Type() != "bool" {
			nameAndType += " " + flag.Value.Type()
		}
		if len(nameAndType) > maxLen {
			maxLen = len(nameAndType)
		}
		flags = append(flags, flagInfo{nameAndType, flag.Usage})
	})

	return flags, maxLen
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

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cobrautils

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type UsageError struct {
	cmd *cobra.Command
	err error
}

func (e UsageError) Error() string {
	return fmt.Sprintf("Usage error: %s", e.err)
}

func ErrWrongArgCount(expected, got int) error {
	return fmt.Errorf("requires %d arg(s), received %d args(s)", expected, got)
}
func ErrMaxArgCount(expected, got int) error {
	return fmt.Errorf("max %d arg(s), received %d args(s)", expected, got)
}

func ErrMinArgCount(expected, got int) error {
	return fmt.Errorf("min %d arg(s), received %d args(s)", expected, got)
}

func ErrRangeArgCount(expectedLow, expectedHigh, got int) error {
	return fmt.Errorf("accepted number of arg(s) is %d to %d, received %d args(s)", expectedLow, expectedHigh, got)
}

func NewUsageError(cmd *cobra.Command, err error) UsageError {
	return UsageError{
		cmd: cmd,
		err: err,
	}
}

func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			_ = cmd.Help() // show full help with flag grouping
			return ErrWrongArgCount(n, len(args))
		}
		return nil
	}
}

func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			_ = cmd.Help() // show full help with flag grouping
			return ErrMaxArgCount(n, len(args))
		}
		return nil
	}
}

func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			_ = cmd.Help() // show full help with flag grouping
			return ErrMinArgCount(n, len(args))
		}
		return nil
	}
}

func RangeArgs(min int, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < min || len(args) > max {
			_ = cmd.Help() // show full help with flag grouping
			return ErrRangeArgCount(min, max, len(args))
		}
		return nil
	}
}

func HandleErrors(err error) {
	if err != nil {
		usageErr, ok := err.(UsageError)
		if ok {
			usageErr.cmd.Println(usageErr.cmd.UsageString())
			usageErr.cmd.Println()
			usageErr.cmd.Println(usageErr)
		} else {
			ux.Logger.PrintToUser("Error: %s", err)
		}
		os.Exit(1)
	}
}

func CommandSuiteUsage(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return NewUsageError(
			cmd,
			fmt.Errorf("invalid subcommand %q", strings.Join(args, " ")),
		)
	}
	err := cmd.Help()
	if err != nil {
		fmt.Println(err)
	}
	return nil
}

func ConfigureRootCmd(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return NewUsageError(cmd, err)
	})
}

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

func NewUsageError(cmd *cobra.Command, err error) UsageError {
	return UsageError{
		cmd: cmd,
		err: err,
	}
}

func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := cobra.ExactArgs(n)(cmd, args)
		if err != nil {
			_ = cmd.Help()
			err = NewUsageError(cmd, err)
		}
		return err
	}
}

func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := cobra.MaximumNArgs(n)(cmd, args)
		if err != nil {
			_ = cmd.Help()
			err = NewUsageError(cmd, err)
		}
		return err
	}
}

func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := cobra.MinimumNArgs(n)(cmd, args)
		if err != nil {
			_ = cmd.Help()
			err = NewUsageError(cmd, err)
		}
		return err
	}
}

func RangeArgs(min int, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := cobra.RangeArgs(min, max)(cmd, args)
		if err != nil {
			_ = cmd.Help()
			err = NewUsageError(cmd, err)
		}
		return err
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

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// avalanche config metrics command
func new1NodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "onenode [enable | disable]",
		Short:        "opt in or out of one-node local network",
		Long:         "set user preference between one-node and five nodes local network",
		RunE:         handle1NodeSettings,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	return cmd
}

func handle1NodeSettings(_ *cobra.Command, args []string) error {
	switch args[0] {
	case constants.Enable:
		err := save1NodePreferences(true)
		if err != nil {
			return err
		}
	case constants.Disable:
		err := save1NodePreferences(false)
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid argument '" + args[0] + "'")
	}
	return nil
}

func save1NodePreferences(enable bool) error {
	return app.Conf.SetConfigValue(constants.Config1NodeEnabledKey, enable)
}

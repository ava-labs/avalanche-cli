// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// avalanche config singlenode command
func newSingleNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "singleNode [enable | disable]",
		Short:        "opt in or out of single node local network",
		Long:         "set user preference between single node and five nodes local network",
		RunE:         handleSingleNodeSettings,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	return cmd
}

func handleSingleNodeSettings(_ *cobra.Command, args []string) error {
	switch args[0] {
	case constants.Enable:
		err := saveSingleNodePreferences(true)
		if err != nil {
			return err
		}
	case constants.Disable:
		err := saveSingleNodePreferences(false)
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid argument '" + args[0] + "'")
	}
	return nil
}

func saveSingleNodePreferences(enable bool) error {
	return app.Conf.SetConfigValue(constants.ConfigSingleNodeEnabledKey, enable)
}

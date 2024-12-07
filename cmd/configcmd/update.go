// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche config metrics command
func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [enable | disable]",
		Short: "opt in or out of update check",
		Long:  "set user preference between update check or not",
		RunE:  handleUpdateSettings,
		Args:  cobrautils.ExactArgs(1),
	}

	return cmd
}

func handleUpdateSettings(_ *cobra.Command, args []string) error {
	switch args[0] {
	case constants.Enable:
		ux.Logger.PrintToUser("Thank you for opting in Avalanche CLI automated update check")
		err := saveUpdatePreferences(true)
		if err != nil {
			return err
		}
	case constants.Disable:
		ux.Logger.PrintToUser("Avalanche CLI automated update check will no longer be performed")
		err := saveUpdatePreferences(false)
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid update argument '" + args[0] + "'")
	}
	return nil
}

func saveUpdatePreferences(enableUpdate bool) error {
	return app.Conf.SetConfigValue(constants.ConfigUpdatesEnabledKey, enableUpdate)
}

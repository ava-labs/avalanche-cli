// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

func handleBooleanSetting(cmd *cobra.Command, key string, args []string) error {
	if len(args) == 0 {
		ux.Logger.PrintToUser(cmd.UsageString())
		ux.Logger.PrintToUser("")
		if app.Conf.GetConfigBoolValue(key) {
			ux.Logger.PrintToUser("Current Setting: Enabled")
		} else {
			ux.Logger.PrintToUser("Current Setting: Disabled")
		}
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("unexpected number of arguments")
	}
	arg := args[0]
	switch arg {
	case constants.Enable:
		if err := app.Conf.SetConfigValue(key, true); err != nil {
			return err
		}
	case constants.Disable:
		if err := app.Conf.SetConfigValue(key, false); err != nil {
			return err
		}
	default:
		return errors.New("Invalid argument '" + arg + "'")
	}
	return nil
}

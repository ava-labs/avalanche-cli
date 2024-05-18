// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func handleBooleanSetting(key string, args []string) error {
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

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"encoding/json"
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche transaction sign
func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "metrics [enable | disable]",
		Short:        "opt in or out of metrics collection",
		Long:         "set user metrics collection preferences",
		RunE:         handleMetricsSettings,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	return cmd
}

func handleMetricsSettings(_ *cobra.Command, args []string) error {
	switch args[0] {
	case constants.Enable:
		ux.Logger.PrintToUser("Thank you for opting in Avalanche CLI usage metrics collection")
		err := saveMetricsPreferences(true)
		if err != nil {
			return err
		}
	case constants.Disable:
		ux.Logger.PrintToUser("Avalanche CLI usage metrics will no longer be collected")
		err := saveMetricsPreferences(false)
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid metrics argument '" + args[0] + "'")
	}
	return nil
}

func saveMetricsPreferences(enableMetrics bool) error {
	config := models.Config{
		MetricsEnabled: enableMetrics,
	}

	jsonBytes, err := json.Marshal(&config)
	if err != nil {
		return err
	}

	return app.WriteConfigFile(jsonBytes)
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche config metrics command
func newAutorizeCloudAccessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "authorize-cloud-access [enable | disable]",
		Short:        "authorize access to cloud resources",
		Long:         "set preferences to authorize access to cloud resources",
		RunE:         handleAutorizeCloudAccess,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	return cmd
}

func handleAutorizeCloudAccess(_ *cobra.Command, args []string) error {
	switch args[0] {
	case constants.Enable:
		ux.Logger.PrintToUser("Thank you for authorizing Avalanche-CLI to access your Cloud account(s)")
		ux.Logger.PrintToUser("By enabling this setting you are authorizing Avalanche-CLI to:")
		ux.Logger.PrintToUser("- Create Cloud instance(s) and other components (such as elastic IPs)")
		ux.Logger.PrintToUser("- Start/Stop Cloud instance(s) and other components (such as elastic IPs) previously created by Avalanche-CLI")
		ux.Logger.PrintToUser("- Delete Cloud instance(s) and other components (such as elastic IPs) previously created by Avalanche-CLI")
		err := saveAutorizeCloudAccessPreferences(true)
		if err != nil {
			return err
		}
	case constants.Disable:
		ux.Logger.PrintToUser("Avalanche-CLI Cloud access has been disabled.")
		ux.Logger.PrintToUser("You can re-enable Cloud access by running 'avalanche config authorize-cloud-access enable'")
		err := saveAutorizeCloudAccessPreferences(false)
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid authorize-cloud-access argument '" + args[0] + "'")
	}
	return nil
}

func saveAutorizeCloudAccessPreferences(enableAccess bool) error {
	return app.Conf.SetConfigValue(constants.ConfigAutorizeCloudAccessKey, enableAccess)
}

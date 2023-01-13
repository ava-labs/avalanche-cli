// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// avalanche subnet upgrade generate
func newUpgradeInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [subnetName]",
		Short: "Installs upgrade bytes onto subnet nodes",
		Long:  `Installs upgrade bytes onto VMs.`, // TODO fix this wording
		RunE:  installCmd,
		Args:  cobra.ExactArgs(1),
	}
	return cmd
}

func installCmd(cmd *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	networkToUpgrade, err := selectNetworkToUpgrade(sc)
	if err != nil {
		return err
	}
	fmt.Println(networkToUpgrade)
	return nil
}

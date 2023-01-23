// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet/upgrades"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ANRclient "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	timestampFormat  = "20060102150405"
	tmpSnapshotInfix = "-tmp-"
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

	switch networkToUpgrade {
	case futureDeployment:
		// in this case, we just are going to generate new update bytes
		return upgradeGenerateCmd(cmd, args)
		// update a locally running network
	case localDeployment:
		return saveAndRestartFromSnapshot(subnetName, sc)
	}

	return nil
}

func saveAndRestartFromSnapshot(subnetName string, sc models.Sidecar) error {
	// For a already deployed subnet, the supported scheme is to
	// save a snapshot, and to load the snapshot with the upgrade
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}
	ctx := binutils.GetAsyncContext()

	blockchainID := sc.Networks[models.Local.String()].BlockchainID
	if blockchainID == ids.Empty {
		return errors.New(
			"failed to find deployment information about this subnet in state - aborting")
	}

	snapName := subnetName + tmpSnapshotInfix + time.Now().Format(timestampFormat)
	app.Log.Debug("saving temporary snapshot for upgrade bytes", zap.String("snapshot-name", snapName))
	_, err = cli.SaveSnapshot(ctx, snapName)
	if err != nil {
		return err
	}
	app.Log.Debug(
		"network stopped and named temporary snapshot created. Now starting the network with given snapshot")

	netUpgradeBytes, err := upgrades.ReadUpgradeFile(subnetName, app.GetSubnetDir())
	if err != nil {
		if err == os.ErrNotExist {
			ux.Logger.PrintToUser("A file with upgrade specs for the given subnet have not been found.")
			ux.Logger.PrintToUser("You may need to first create it with the `avalanche subnet update generate` command")
		}
		return err
	}

	// TODO input validation
	netUpgradeConfs := map[string]string{
		blockchainID.String(): string(netUpgradeBytes),
	}

	opts := ANRclient.WithUpgradeConfigs(netUpgradeConfs)
	_, err = cli.LoadSnapshot(ctx, snapName, opts)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Network restarted and upgrade bytes have been applied to running nodes")
	return nil
}

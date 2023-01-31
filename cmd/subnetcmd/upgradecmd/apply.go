// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
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

var errNotYetImplemented = errors.New("not yet implemented")

// avalanche subnet upgrade apply
func newUpgradeApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [subnetName]",
		Short: "Apply upgrade bytes onto subnet nodes",
		Long:  `Apply generated upgrade bytes to running subnet nodes to trigger a network upgrade`,
		RunE:  applyCmd,
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&useConfig, "config", false, "create upgrade config for future subnet deployments (same as generate)")
	cmd.Flags().BoolVar(&useLocal, "local", false, "apply upgrade existing `local` deployment")
	cmd.Flags().BoolVar(&useFuji, "fuji", false, "apply upgrade existing `fuji` deployment (alias for `testnet`)")
	cmd.Flags().BoolVar(&useFuji, "testnet", false, "apply upgrade existing `testnet` deployment (alias for `fuji`)")
	cmd.Flags().BoolVar(&useMainnet, "mainnet", false, "apply upgrade existing `mainnet` deployment")

	return cmd
}

func applyCmd(_ *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	networkToUpgrade, err := selectNetworkToUpgrade(sc, []string{})
	if err != nil {
		return err
	}

	switch networkToUpgrade {
	// update a locally running network
	case localDeployment:
		return applyLocalNetworkUpgrade(subnetName, sc)
	case fujiDeployment:
		return errNotYetImplemented
	case mainnetDeployment:
		return errNotYetImplemented
	}

	return nil
}

func applyLocalNetworkUpgrade(subnetName string, sc models.Sidecar) error {
	// For a already deployed subnet, the supported scheme is to
	// save a snapshot, and to load the snapshot with the upgrade

	// first let's check update bytes actually exist
	netUpgradeBytes, err := upgrades.ReadUpgradeFile(subnetName, app)
	if err != nil {
		if err == os.ErrNotExist {
			ux.Logger.PrintToUser("No file with upgrade specs for the given subnet has been found")
			ux.Logger.PrintToUser("You may need to first create it with the `avalanche subnet upgrade generate` command or import it")
			ux.Logger.PrintToUser("Aborting this command. No changes applied")
		}
		return err
	}

	upgrades, err := validateUpgradeBytes(netUpgradeBytes)
	if err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}
	ctx := binutils.GetAsyncContext()

	// get the blockchainID from the sidecar
	blockchainID := sc.Networks[models.Local.String()].BlockchainID
	if blockchainID == ids.Empty {
		return errors.New(
			"failed to find deployment information about this subnet in state - aborting")
	}

	// save a temporary snapshot
	snapName := subnetName + tmpSnapshotInfix + time.Now().Format(timestampFormat)
	app.Log.Debug("saving temporary snapshot for upgrade bytes", zap.String("snapshot-name", snapName))
	_, err = cli.SaveSnapshot(ctx, snapName)
	if err != nil {
		return err
	}
	app.Log.Debug(
		"network stopped and named temporary snapshot created. Now starting the network with given snapshot")

	netUpgradeConfs := map[string]string{
		blockchainID.String(): string(netUpgradeBytes),
	}
	// restart the network setting the upgrade bytes file
	opts := ANRclient.WithUpgradeConfigs(netUpgradeConfs)
	_, err = cli.LoadSnapshot(ctx, snapName, opts)
	if err != nil {
		return err
	}

	// TODO as noted elsewhere, we need to extract the health polling from the deployer
	// we shouldn't have to create the deployer here just to poll for healthy
	sd := subnet.NewLocalDeployer(app, "", "")
	clusterInfo, err := sd.WaitForHealthy(ctx, cli, constants.HealthCheckInterval)
	if err != nil {
		return fmt.Errorf("failed waiting for network to become healthy: %w", err)
	}

	endpoints := subnet.GetEndpoints(clusterInfo)

	fmt.Println()
	if len(endpoints) > 0 {
		ux.Logger.PrintToUser("Network restarted and ready to use. Upgrade bytes have been applied to running nodes at these endpoints.")

		nextUpgrade, err := getEarliestTimestamp(upgrades)
		// this should not happen anymore at this point...
		if err != nil {
			app.Log.Warn("looks like the upgrade went well, but we failed getting the timestamp of the next upcoming upgrade: %w")
		}
		ux.Logger.PrintToUser("The next upgrade will go into effect %s", time.Unix(nextUpgrade, 0).Local().Format(constants.TimeParseLayout))
		ux.PrintTableEndpoints(clusterInfo)
	}
	return nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ANRclient "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	timestampFormat  = "20060102150405"
	tmpSnapshotInfix = "-tmp-"
)

var (
	ErrNetworkNotStartedOutput = "No local network running. Please start the network first."
	ErrSubnetNotDeployedOutput = "Looks like this subnet has not been deployed to a local network yet."

	errNotYetImplemented    = errors.New("not yet implemented")
	errSubnetNotYetDeployed = errors.New("subnet not yet deployed")
)

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

// applyLocalNetworkUpgrade:
// * if subnet NOT deployed (`network status`):
// *   Stop the apply command and print a message suggesting to deploy first
// * if subnet deployed:
// *   if never upgraded before, apply
// *   if upgraded before, and this upgrade contains the same upgrade as before (.lock)
// *     if has new valid upgrade on top, apply
// *     if the same, print info and do nothing
// *   if upgraded before, but this upgrade is not cumulative (append-only)
// *     fail the apply, print message

// For a already deployed subnet, the supported scheme is to
// save a snapshot, and to load the snapshot with the upgrade
func applyLocalNetworkUpgrade(subnetName string, sc models.Sidecar) error {
	// if there's no entry in the Sidecar, we assume there hasn't been a deploy yet
	networkKey := models.Local.String()
	if sc.Networks[networkKey] == (models.NetworkData{}) {
		return subnetNotYetDeployed()
	}
	// let's check update bytes actually exist
	netUpgradeBytes, err := app.ReadUpgradeFile(subnetName)
	if err != nil {
		if err == os.ErrNotExist {
			ux.Logger.PrintToUser("No file with upgrade specs for the given subnet has been found")
			ux.Logger.PrintToUser("You may need to first create it with the `avalanche subnet upgrade generate` command or import it")
			ux.Logger.PrintToUser("Aborting this command. No changes applied")
		}
		return err
	}

	// read the lock file right away
	lockUpgradeBytes, err := app.ReadLockUpgradeFile(subnetName)
	if err != nil {
		// if the file doesn't exist, that's ok
		if !os.IsNotExist(err) {
			return err
		}
	}

	// validate the upgrade bytes files
	precmpUpgrades, err := validateUpgradeBytes(netUpgradeBytes, lockUpgradeBytes)
	if err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		ux.Logger.PrintToUser(ErrNetworkNotStartedOutput)
		return err
	}
	ctx := binutils.GetAsyncContext()

	// first let's get the status
	status, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser(ErrNetworkNotStartedOutput)
			return err
		}
		return err
	}

	// confirm in the status that the subnet actually is deployed and running
	deployed := false
	subnets := status.ClusterInfo.GetSubnets()
	for _, s := range subnets {
		if s == sc.Networks[networkKey].SubnetID.String() {
			deployed = true
			break
		}
	}

	if !deployed {
		return subnetNotYetDeployed()
	}

	// get the blockchainID from the sidecar
	blockchainID := sc.Networks[networkKey].BlockchainID
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

	clusterInfo, err := subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed waiting for network to become healthy: %w", err)
	}

	fmt.Println()
	if subnet.HasEndpoints(clusterInfo) {
		ux.Logger.PrintToUser("Network restarted and ready to use. Upgrade bytes have been applied to running nodes at these endpoints.")

		nextUpgrade, err := getEarliestTimestamp(precmpUpgrades)
		// this should not happen anymore at this point...
		if err != nil {
			app.Log.Warn("looks like the upgrade went well, but we failed getting the timestamp of the next upcoming upgrade: %w")
		}
		ux.Logger.PrintToUser("The next upgrade will go into effect %s", time.Unix(nextUpgrade, 0).Local().Format(constants.TimeParseLayout))
		ux.PrintTableEndpoints(clusterInfo)

		// it seems all went well this far, now we try to write/update the lock file
		// if this fails, we probably don't want to cause an error to the user?
		// so we are silently failing, just write a log entry
		wrapper := params.UpgradeConfig{
			PrecompileUpgrades: precmpUpgrades,
		}
		jsonBytes, err := json.Marshal(wrapper)
		if err != nil {
			app.Log.Debug("failed to marshaling upgrades lock file content", zap.Error(err))
			return nil
		}
		if err := app.WriteLockUpgradeFile(subnetName, jsonBytes); err != nil {
			app.Log.Debug("failed to write upgrades lock file", zap.Error(err))
		}

		return nil
	}

	return errors.New("unexpected network size of zero nodes")
}

func subnetNotYetDeployed() error {
	ux.Logger.PrintToUser(ErrSubnetNotDeployedOutput)
	ux.Logger.PrintToUser("Please deploy this network first.")
	return errSubnetNotYetDeployed
}

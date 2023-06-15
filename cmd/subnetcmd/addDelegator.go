// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

// avalanche subnet deploy
func newAddPermissionlessDelegatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addPermissionlessDelegator [subnetName]",
		Short: "Allow a node join an existing subnet validator as a delegator",
		Long: `The subnet addDelegator enables a node (the delegator) to stake 
AVAX and specify a validator (the delegatee) to validate on their behalf. The 
delegatee has an increased probability of being sampled by other validators 
(weight) in proportion to the stake delegated to them.

The delegatee charges a fee to the delegator; the former receives a percentage 
of the delegator’s validation reward (if any.) A transaction that delegates 
stake has no fee.

The delegation period must be a subset of the period that the delegatee 
validates the Primary Network.

To add a node as a delegator, you first need to provide
the subnetID and the validator's unique NodeID. The command then prompts
for the validation start time, duration, and stake weight. You can bypass
these prompts by providing the values with flags.`,
		SilenceUsage: true,
		RunE:         addPermissionlessDelegator,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "join on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "join on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "join on `mainnet`")
	cmd.Flags().BoolVar(&deployLocal, "local", false, "join on `local`")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "amount of tokens to stake")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "start time that delegator starts delegating")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long delegator should delegate for after start time")

	return cmd
}

func addPermissionlessDelegator(_ *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	var (
		nodeID ids.NodeID
		start  time.Time
	)

	var network models.Network
	switch {
	case deployLocal:
		network = models.Local
	case deployMainnet:
		network = models.Mainnet
	case deployTestnet:
		network = models.Fuji
	}

	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network for the node to be a delegator in.",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	switch network {
	case models.Fuji:
		return errors.New("addPermissionlessDelegator is not yet supported on Fuji network")
	case models.Mainnet:
		return errors.New("addPermissionlessDelegator is not yet supported on Mainnet")
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if !checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf("%s is not an elastic subnet", subnetName)
	}
	nodeID, err = promptNodeIDToAdd(sc.Networks[network.String()].SubnetID, false, network)
	if err != nil {
		return err
	}
	stakedTokenAmount, err := promptStakeAmount(subnetName, false, network)
	if err != nil {
		return err
	}
	start, stakeDuration, err := getTimeParameters(network, nodeID, false)
	if err != nil {
		return err
	}
	endTime := start.Add(stakeDuration)
	ux.Logger.PrintToUser("Inputs complete, issuing transaction addPermissionlessDelegatorTx...")
	ux.Logger.PrintToUser("")

	assetID := sc.ElasticSubnet[network.String()].AssetID
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	subnetID := sc.Networks[network.String()].SubnetID
	txID, err := subnet.IssueAddPermissionlessDelegatorTx(keyChain, subnetID, nodeID, stakedTokenAmount, assetID, uint64(start.Unix()), uint64(endTime.Unix()))
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node successfully added as delegator!")
	ux.Logger.PrintToUser("TX ID: %s", txID.String())
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.UTC().Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", endTime.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("Stake Amount: %d", stakedTokenAmount)
	return nil
}

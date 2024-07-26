// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/spf13/cobra"
)

var addPermissionlessDelegatorSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Fuji, networkoptions.Mainnet}

// avalanche blockchain addPermissionlessDelegator
func newAddPermissionlessDelegatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addPermissionlessDelegator [blockchainName]",
		Short: "Allow a node join an existing subnet validator as a delegator",
		Long: `The blockchain addDelegator enables a node (the delegator) to stake 
AVAX and specify a validator (the delegatee) to validate on their behalf. The 
delegatee has an increased probability of being sampled by other validators 
(weight) in proportion to the stake delegated to them.

The delegatee charges a fee to the delegator; the former receives a percentage 
of the delegator’s validation reward (if any.) A transaction that delegates 
stake has no fee.

The delegation period must be a subset of the period that the delegatee 
validates the Primary Network.

To add a node as a delegator, you first need to provide
the validator's unique NodeID. The command then prompts
for the validation start time, duration, and stake weight. You can bypass
these prompts by providing the values with flags.`,
		RunE: addPermissionlessDelegator,
		Args: cobrautils.ExactArgs(1),
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, addPermissionlessDelegatorSupportedNetworkOptions)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to delegate to")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "amount of tokens to stake")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "start time that delegator starts delegating")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long delegator should delegate for after start time")

	return cmd
}

func addPermissionlessDelegator(_ *cobra.Command, args []string) error {
	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	blockchainName := chains[0]
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		addPermissionlessDelegatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		subnetID = sc.Networks[models.Local.String()].SubnetID
	}
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	nodeID, err := promptNodeIDToAdd(subnetID, false, network)
	if err != nil {
		return err
	}
	stakedTokenAmount, err := promptStakeAmount(blockchainName, false, network)
	if err != nil {
		return err
	}
	start, stakeDuration, err := getTimeParameters(network, nodeID, false)
	if err != nil {
		return err
	}
	endTime := start.Add(stakeDuration)

	switch network.Kind {
	case models.Local:
		return handleAddPermissionlessDelegatorLocal(blockchainName, network, nodeID, stakedTokenAmount, start, endTime)
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		return errors.New("addPermissionlessDelegator is not yet supported on Mainnet")
	}

	// get keychain accessor
	fee := network.GenesisParams().AddSubnetDelegatorFee
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, fee)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	recipientAddr := kc.Addresses().List()[0]
	deployer := subnet.NewPublicDeployer(app, kc, network)
	assetID, err := getSubnetAssetID(subnetID, network)
	if err != nil {
		return err
	}
	txID, err := deployer.AddPermissionlessDelegator(subnetID, assetID, nodeID, stakedTokenAmount, uint64(start.Unix()), uint64(endTime.Unix()), recipientAddr)
	if err != nil {
		return err
	}
	printAddPermissionlessDelOutput(txID, nodeID, network, start, endTime, stakedTokenAmount)
	return nil
}

func printAddPermissionlessDelOutput(txID ids.ID, nodeID ids.NodeID, network models.Network, start time.Time, endTime time.Time, stakedTokenAmount uint64) {
	ux.Logger.PrintToUser("Node successfully added as delegator!")
	ux.Logger.PrintToUser("TX ID: %s", txID.String())
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.Name())
	ux.Logger.PrintToUser("Start time: %s", start.UTC().Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", endTime.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("Stake Amount: %d", stakedTokenAmount)
}

func handleAddPermissionlessDelegatorLocal(blockchainName string, network models.Network, nodeID ids.NodeID,
	stakedTokenAmount uint64, start time.Time, endTime time.Time,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	if !checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf("%s is not an elastic subnet", blockchainName)
	}
	ux.Logger.PrintToUser("Inputs complete, issuing transaction addPermissionlessDelegatorTx...")
	ux.Logger.PrintToUser("")
	assetID := sc.ElasticSubnet[network.Name()].AssetID
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	subnetID := sc.Networks[network.Name()].SubnetID
	txID, err := subnet.IssueAddPermissionlessDelegatorTx(keyChain, subnetID, nodeID, stakedTokenAmount, assetID, uint64(start.Unix()), uint64(endTime.Unix()))
	if err != nil {
		return err
	}
	printAddPermissionlessDelOutput(txID, nodeID, network, start, endTime, stakedTokenAmount)
	return nil
}

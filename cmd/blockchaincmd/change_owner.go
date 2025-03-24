// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/flags"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

// avalanche blockchain changeOwner
func newChangeOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeOwner [blockchainName]",
		Short: "Change owner of the blockchain",
		Long:  `The blockchain changeOwner changes the owner of the deployed Blockchain.`,
		RunE:  changeOwner,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet]")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet]")
	return cmd
}

func changeOwner(_ *cobra.Command, args []string) error {
	blockchainName := args[0]

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	// TODO: will estimate fee in subsecuent PR
	fee := uint64(0)
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		"pay fees",
		network,
		keyName,
		useEwoq,
		useLedger,
		ledgerAddresses,
		fee,
	)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	if flags.NonSovFlags.OutputTxPath != "" {
		if utils.FileExists(flags.NonSovFlags.OutputTxPath) {
			return fmt.Errorf("outputTxPath %q already exists", flags.NonSovFlags.OutputTxPath)
		}
	}

	_, err = ValidateSubnetNameAndGetChains([]string{blockchainName})
	if err != nil {
		return err
	}

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	_, currentControlKeys, currentThreshold, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	currentSubnetOwnerFlags := flags.SubnetFlags{
		ControlKeys:    currentControlKeys,
		Threshold:      currentThreshold,
		SubnetAuthKeys: flags.NonSovFlags.SubnetAuthKeys,
		OutputTxPath:   flags.NonSovFlags.OutputTxPath,
	}
	// flags.NonSovFlags in this example contains control keys, threshold for the new owners of the blockchain
	// subnet auth keys flags provided is for the old blockchain, and thus must be set empty for the new blockchain owners
	// since it is only relevant for old subnet owners
	flags.NonSovFlags.SubnetAuthKeys = []string{}
	// same case for output tx path, we'll set it to empty since this is only relevant for old subnet owners
	flags.NonSovFlags.OutputTxPath = ""

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(currentControlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for add validator tx signing
	if currentSubnetOwnerFlags.SubnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, currentSubnetOwnerFlags); err != nil {
			return err
		}
	} else {
		err = prompts.SetSubnetAuthKeys(app.Prompt, kcKeys, &currentSubnetOwnerFlags)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your auth keys for add validator tx creation: %s", currentSubnetOwnerFlags.SubnetAuthKeys)

	err = promptOwners(
		kc,
		&flags.NonSovFlags,
		false,
	)
	if err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)
	isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.TransferSubnetOwnership(
		currentSubnetOwnerFlags,
		subnetID,
		flags.NonSovFlags.ControlKeys,
		flags.NonSovFlags.Threshold,
	)
	if err != nil {
		return err
	}
	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Transfer Blockchain Ownership",
			tx,
			blockchainName,
			currentSubnetOwnerFlags,
			remainingSubnetAuthKeys,
			false,
		); err != nil {
			return err
		}
	}
	return nil
}

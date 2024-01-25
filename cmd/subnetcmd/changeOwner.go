// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

// avalanche subnet changeOwner
func newChangeOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeOwner [subnetName]",
		Short: "Change owner of the subnet",
		Long: `The subnet changeOwner changes the owner of the deployed Subnet.

This command currently only works on Subnets deployed to Devnet, Fuji or Mainnet.`,
		SilenceUsage: true,
		RunE:         changeOwner,
		Args:         cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet]")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet]")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVar(&deployLocal, "local", false, "change subnet owner on `local`")
	cmd.Flags().BoolVar(&deployDevnet, "devnet", false, "change subnet owner on `devnet`")
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "change subnet owner on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "change subnet owner on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "change subnet owner on `mainnet`")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate transfer subnet ownership tx")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use the fee-paying key as control key")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may make subnet changes")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to make subnet changes")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the transfer subnet ownership tx")
	return cmd
}

func changeOwner(_ *cobra.Command, args []string) error {
	subnetName := args[0]

	network, err := GetNetworkFromCmdLineFlags(
		deployLocal,
		deployDevnet,
		deployTestnet,
		deployMainnet,
		endpoint,
		true,
		[]models.NetworkKind{models.Local, models.Devnet, models.Fuji, models.Mainnet},
	)
	if err != nil {
		return err
	}

	fee := network.GenesisParams().TxFee
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		constants.PayTxsFeesMsg,
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

	if outputTxPath != "" {
		if utils.FileExists(outputTxPath) {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	_, err = ValidateSubnetNameAndGetChains([]string{subnetName})
	if err != nil {
		return err
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	transferSubnetOwnershipTxID := sc.Networks[network.Name()].TransferSubnetOwnershipTxID

	currentControlKeys, currentThreshold, err := txutils.GetOwners(network, subnetID, transferSubnetOwnershipTxID)
	if err != nil {
		return err
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(currentControlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for add validator tx signing
	if subnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, subnetAuthKeys, currentControlKeys, currentThreshold); err != nil {
			return err
		}
	} else {
		subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, kcKeys, currentControlKeys, currentThreshold)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your subnet auth keys for add validator tx creation: %s", subnetAuthKeys)

    controlKeys, threshold, err = promptOwners(
        kc,
        controlKeys,
        sameControlKey,
        threshold,
        nil,
    )
    if err != nil {
        return err
    }

	fmt.Println(currentControlKeys)
	fmt.Println(currentThreshold)
	fmt.Println(controlKeys)
	fmt.Println(threshold)

    deployer := subnet.NewPublicDeployer(app, kc, network)
    isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.TransferSubnetOwnership(
        currentControlKeys,
        subnetAuthKeys,
        subnetID,
        controlKeys,
        threshold,
    )
    if err != nil {
        return err
    }
    if !isFullySigned {
        if err := SaveNotFullySignedTx(
            "Transfer Subnet Ownership",
            tx,
            subnetName,
            subnetAuthKeys,
            remainingSubnetAuthKeys,
            outputTxPath,
            false,
        ); err != nil {
            return err
        }
    }

    return err
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

// avalanche blockchain addValidator
func newChangeWeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeWeight [blockchainName]",
		Short: "Changes the weight of a Subnet validator",
		Long: `The blockchain changeWeight command changes the weight of a Subnet Validator.

The Subnet has to be a Proof of Authority Subnet-Only Validator Subnet.`,
		RunE: setWeight,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, addValidatorSupportedNetworkOptions)

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&weight, "weight", constants.NonBootstrapValidatorWeight, "set the new staking weight of the validator")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func setWeight(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	err := prompts.ValidateNodeID(nodeIDStr)
	if err != nil {
		return err
	}

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}

	networkOptionsList := networkoptions.GetNetworkFromSidecar(sc, removeValidatorSupportedNetworkOptions)
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkOptionsList,
		"",
	)
	if err != nil {
		return err
	}

	if network.Kind == models.Mainnet && sc.Sovereign {
		return errNotSupportedOnMainnet
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

	switch network.Kind {
	case models.Devnet:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
			if err != nil {
				return err
			}
		}
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// get keychain accesor
	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, fee)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	var nodeID ids.NodeID
	if nodeIDStr == "" {
		nodeID, err = PromptNodeID("add as a blockchain validator")
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
	if err != nil {
		// just warn the user, don't fail
		ux.Logger.PrintToUser("failed to check if node is a validator on the subnet: %s", err)
	} else if !isValidator {
		// this is actually an error
		return fmt.Errorf("node %s is not a validator on subnet %s", nodeID, subnetID)
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)

	// first remove the validator from subnet
	err = removeValidatorSOV(deployer,
		network,
		blockchainName,
		nodeID,
		0, // automatic uptime
		isBootstrapValidatorForNetwork(nodeID, sc.Networks[network.Name()]),
		false, // don't force
	)
	if err != nil {
		return err
	}

	// TODO: we need to wait for the balance from the removed validator to arrive in changeAddr
	// set arbitrary time.sleep here?

	weight, err = app.Prompt.CaptureWeight("What weight would you like to assign to the validator?")
	if err != nil {
		return err
	}

	balance, err = getValidatorBalanceFromPChain()
	if err != nil {
		return err
	}

	publicKey, pop, err = getBLSInfoFromPChain()
	if err != nil {
		return err
	}

	remainingBalanceOwnerAddr, err = getChangeAddrFromPChain()
	if err != nil {
		return fmt.Errorf("failure parsing change owner address: %w", err)
	}

	// add back validator to subnet with updated weight
	return CallAddValidator(deployer, network, kc, blockchainName, nodeID.String(), publicKey, pop)
}

// getValidatorBalanceFromPChain gets remaining balance of validator from p chain
func getValidatorBalanceFromPChain() (uint64, error) {
	return 0, nil
}

// getBLSInfoFromPChain gets BLS public key and pop from info api
func getBLSInfoFromPChain() (string, string, error) {
	return "", "", nil
}

// getChangeAddrFromPChain gets validator change addr from info api
func getChangeAddrFromPChain() (string, error) {
	return "", nil
}

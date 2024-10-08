// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"os"

	warpPlatformVM "github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/spf13/cobra"
)

var removeValidatorSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

// avalanche blockchain removeValidator
func newRemoveValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "removeValidator [blockchainName]",
		Short: "Remove a permissioned validator from your blockchain's subnet",
		Long: `The blockchain removeValidator command stops a whitelisted, subnet network validator from
validating your deployed Blockchain.

To remove the validator from the Subnet's allow list, provide the validator's unique NodeID. You can bypass
these prompts by providing the values with flags.`,
		RunE: removeValidator,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, removeValidatorSupportedNetworkOptions)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "(for non-SOV blockchain only) control keys that will be used to authenticate the removeValidator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "(for non-SOV blockchain only) file path of the removeValidator tx")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().BoolVar(&sovereign, "sovereign", true, "set to false if removing validator in a non-sovereign blockchain")
	return cmd
}

func removeValidator(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	_, err := ids.NodeIDFromString(args[1])
	if err != nil {
		return err
	}
	nodeIDStr = args[1]

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		removeValidatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if !sovereign {
		if outputTxPath != "" {
			return errors.New("--output-tx-path flag cannot be used for non-SOV (Subnet-Only Validators) blockchains")
		}

		if len(subnetAuthKeys) > 0 {
			return errors.New("--subnetAuthKeys flag cannot be used for non-SOV (Subnet-Only Validators) blockchains")
		}
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

	_, err = ValidateSubnetNameAndGetChains([]string{blockchainName})
	if err != nil {
		return err
	}

	switch network.Kind {
	case models.Local:
		if !sovereign {
			return removeFromLocalNonSOV(blockchainName)
		}
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

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	var nodeID ids.NodeID
	if nodeIDStr == "" {
		nodeID, err = PromptNodeID("remove as a blockchain validator")
		if err != nil {
			return err
		}
	} else {
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	// check that this guy actually is a validator on the subnet
	isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
	if err != nil {
		// just warn the user, don't fail
		ux.Logger.PrintToUser("failed to check if node is a validator on the subnet: %s", err)
	} else if !isValidator {
		// this is actually an error
		return fmt.Errorf("node %s is not a validator on subnet %s", nodeID, subnetID)
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)
	if !sovereign {
		return removeValidatorNonSOV(deployer, network, subnetID, kc, blockchainName, nodeID)
	}
	return removeValidatorSOV(deployer, network, subnetID, nodeID)
}

// TODO: implement getMinNonce
// getMinNonce gets minNonce associated with the validationID from P-Chain
func getMinNonce(validationID [32]byte) (uint64, error) {
	return 0, nil
}

// TODO: implement getValidationID
// get validation ID for a node from P Chain
func getValidationID(nodeID ids.NodeID) [32]byte {
	return [32]byte{}
}

// TODO: implement generateWarpMessageRemoveValidator
func generateWarpMessageRemoveValidator(validationID [32]byte, nonce, weight uint64) (warpPlatformVM.Message, error) {
	return warpPlatformVM.Message{}, nil
}

func removeValidatorSOV(deployer *subnet.PublicDeployer, network models.Network, subnetID ids.ID, nodeID ids.NodeID) error {
	validators, err := subnet.GetPublicSubnetValidators(subnetID, network)
	if err != nil {
		return err
	}
	if len(validators) == 1 {
		return fmt.Errorf("cannot remove the only validator left in blockchain")
	}

	validationID := getValidationID(nodeID)
	minNonce, err := getMinNonce(validationID)
	if err != nil {
		return err
	}
	// automatically increase nonce by 1
	message, err := generateWarpMessageRemoveValidator(validationID, minNonce+1, 0)
	if err != nil {
		return err
	}
	tx, err := deployer.SetL1ValidatorWeight(message)
	if err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Set Subnet Validator Weight to 0 Tx ID: %s", tx.ID())
	return nil
}

func removeValidatorNonSOV(deployer *subnet.PublicDeployer, network models.Network, subnetID ids.ID, kc *keychain.Keychain, blockchainName string, nodeID ids.NodeID) error {
	isPermissioned, controlKeys, threshold, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	if !isPermissioned {
		return ErrNotPermissionedSubnet
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for add validator tx signing
	if subnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, subnetAuthKeys, controlKeys, threshold); err != nil {
			return err
		}
	} else {
		subnetAuthKeys, err = prompts.GetSubnetAuthKeys(app.Prompt, kcKeys, controlKeys, threshold)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your subnet auth keys for remove validator tx creation: %s", subnetAuthKeys)

	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.Name())
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to remove the specified validator...")

	isFullySigned, tx, remainingSubnetAuthKeys, err := deployer.RemoveValidator(
		controlKeys,
		subnetAuthKeys,
		subnetID,
		nodeID,
	)
	if err != nil {
		return err
	}
	if !isFullySigned {
		if err := SaveNotFullySignedTx(
			"Remove Validator",
			tx,
			blockchainName,
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

func removeFromLocalNonSOV(blockchainName string) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[models.Local.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	// Get NodeIDs of all validators on the subnet
	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return err
	}

	// construct list of validators to choose from
	validatorList := make([]string, len(validators))
	for i, v := range validators {
		validatorList[i] = v.NodeID.String()
	}

	// Convert NodeID string to NodeID type
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}

	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	_, err = subnet.IssueRemoveSubnetValidatorTx(keyChain, subnetID, nodeID)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Validator removed")

	return nil
}

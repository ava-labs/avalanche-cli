// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/validatorcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var changeWeightSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

// avalanche blockchain addValidator
func newChangeWeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeWeight [blockchainName]",
		Short: "Changes the weight of a L1 validator",
		Long: `The blockchain changeWeight command changes the weight of a L1 Validator.

The L1 has to be a Proof of Authority L1.`,
		RunE: setWeight,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, changeWeightSupportedNetworkOptions)

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&weight, "weight", 0, "set the new staking weight of the validator")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func setWeight(_ *cobra.Command, args []string) error {
	blockchainName := args[0]

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}

	networkOptionsList := networkoptions.GetNetworkFromSidecar(sc, changeWeightSupportedNetworkOptions)
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

	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		"to pay for transaction fees on P-Chain",
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

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	var nodeID ids.NodeID
	if nodeIDStr == "" {
		nodeID, err = PromptNodeID("change weight")
		if err != nil {
			return err
		}
	} else {
		if err := prompts.ValidateNodeID(nodeIDStr); err != nil {
			return err
		}
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	isValidator, err := validatorcmd.IsL1Validator(network, subnetID, nodeID)
	if err != nil {
		// just warn the user, don't fail
		ux.Logger.PrintToUser("failed to check if node is a validator: %s", err)
	} else if !isValidator {
		// this is actually an error
		return fmt.Errorf("node %s is not a validator for blockchain %s", nodeID, subnetID)
	}

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	if rpcURL == "" {
		rpcURL, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return err
		}
	}

	validationID, err := validatorcmd.GetValidationID(rpcURL, nodeID)
	if err != nil {
		return err
	}

	vdrInfo, err := validatorcmd.GetL1ValidatorInfo(network, validationID)
	if err != nil {
		return err
	}

	if weight == 0 {
		ux.Logger.PrintToUser("Current validator weight is %d", vdrInfo.Weight)
		weight, err = app.Prompt.CaptureWeight("What weight would you like to assign to the validator?")
		if err != nil {
			return err
		}
	}

	fmt.Printf("%#v\n", vdrInfo.PublicKey)
	*vdrInfo.PublicKey
	deployer := subnet.NewPublicDeployer(app, kc, network)

	return CallAddValidator(deployer, network, kc, blockchainName, nodeID.String(), publicKey, pop)

	return nil

	// first remove the validator from subnet
	err = removeValidatorSOV(
		deployer,
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

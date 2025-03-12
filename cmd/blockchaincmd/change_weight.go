// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/spf13/cobra"
)

var newWeight uint64

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
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&newWeight, "weight", 0, "set the new staking weight of the validator")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "gather node id/bls from publicly available avalanchego apis on the given endpoint")
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

	networkOptionsList := networkoptions.GetNetworkFromSidecar(sc, networkoptions.DefaultSupportedNetworkOptions)
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

	// TODO: will estimate fee in subsecuent PR
	fee := uint64(0)
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

	if nodeEndpoint != "" {
		nodeIDStr, publicKey, pop, err = utils.GetNodeID(nodeEndpoint)
		if err != nil {
			return err
		}
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

	isValidator, err := validator.IsValidator(network.SDKNetwork(), subnetID, nodeID)
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
	if sc.Networks[network.Name()].ValidatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}
	validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress
	validationID, err := validator.GetValidationID(rpcURL, nodeID, validatorManagerAddress)
	if err != nil {
		return err
	}

	validatorInfo, err := validator.GetValidatorInfo(network.SDKNetwork(), validationID)
	if err != nil {
		return err
	}

	totalWeight, err := validator.GetTotalWeight(network.SDKNetwork(), subnetID)
	if err != nil {
		return err
	}

	allowedChange := float64(totalWeight) * constants.MaxL1TotalWeightChange

	if float64(validatorInfo.Weight) > allowedChange {
		return fmt.Errorf("can't make change: current validator weight %d exceeds max allowed weight change of %d", validatorInfo.Weight, uint64(allowedChange))
	}

	allowedChange = float64(totalWeight-validatorInfo.Weight) * constants.MaxL1TotalWeightChange

	if newWeight == 0 {
		ux.Logger.PrintToUser("Current validator weight is %d", validatorInfo.Weight)
		newWeight, err = app.Prompt.CaptureWeight(
			"What weight would you like to assign to the validator?",
			func(v uint64) error {
				if v > uint64(allowedChange) {
					return fmt.Errorf("weight exceeds max allowed weight change of %d", uint64(allowedChange))
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
	}

	if float64(newWeight) > allowedChange {
		return fmt.Errorf("can't make change: desired validator weight %d exceeds max allowed weight change of %d", newWeight, uint64(allowedChange))
	}

	publicKey, err = formatting.Encode(formatting.HexNC, bls.PublicKeyToCompressedBytes(validatorInfo.PublicKey))
	if err != nil {
		return err
	}

	if pop == "" {
		_, pop, err = promptProofOfPossession(false, true)
		if err != nil {
			return err
		}
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)

	var remainingBalanceOwnerAddr, disableOwnerAddr string
	hrp := key.GetHRP(network.ID)
	if validatorInfo.RemainingBalanceOwner != nil && len(validatorInfo.RemainingBalanceOwner.Addrs) > 0 {
		remainingBalanceOwnerAddr, err = address.Format("P", hrp, validatorInfo.RemainingBalanceOwner.Addrs[0][:])
		if err != nil {
			return err
		}
	}
	if validatorInfo.DeactivationOwner != nil && len(validatorInfo.DeactivationOwner.Addrs) > 0 {
		disableOwnerAddr, err = address.Format("P", hrp, validatorInfo.DeactivationOwner.Addrs[0][:])
		if err != nil {
			return err
		}
	}

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

	balance := validatorInfo.Balance
	if validatorInfo.RemainingBalanceOwner != nil && len(validatorInfo.RemainingBalanceOwner.Addrs) > 0 {
		availableBalance, err := utils.GetNetworkBalance([]ids.ShortID{validatorInfo.RemainingBalanceOwner.Addrs[0]}, network.Endpoint)
		if err != nil {
			ux.Logger.RedXToUser("failure checking remaining balance of validator: %s. continuing with default value", err)
		} else if availableBalance < balance {
			balance = availableBalance
		}
	}

	// add back validator to subnet with updated weight
	return CallAddValidator(
		deployer,
		network,
		kc,
		blockchainName,
		subnetID,
		nodeID.String(),
		publicKey,
		pop,
		newWeight,
		float64(balance)/float64(units.Avax),
		remainingBalanceOwnerAddr,
		disableOwnerAddr,
		sc,
	)
}

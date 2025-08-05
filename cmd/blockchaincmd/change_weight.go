// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	newWeight         uint64
	initiateTxHash    string
	changeWeightFlags BlockchainChangeWeightFlags
)

type BlockchainChangeWeightFlags struct {
	RPC         string
	SigAggFlags flags.SignatureAggregatorFlags
}

// avalanche blockchain changeWeight
func newChangeWeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeWeight [blockchainName]",
		Short: "Changes the weight of a L1 validator",
		Long: `The blockchain changeWeight command changes the weight of a L1 Validator.

The L1 has to be a Proof of Authority L1.`,
		RunE:    setWeight,
		PreRunE: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	flags.AddRPCFlagToCmd(cmd, app, &changeWeightFlags.RPC)
	sigAggGroup := flags.AddSignatureAggregatorFlagsToCmd(cmd, &changeWeightFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&newWeight, "weight", 0, "set the new staking weight of the validator")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "gather node id/bls from publicly available avalanchego apis on the given endpoint")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().BoolVar(&externalValidatorManagerOwner, "external-evm-signature", false, "set this value to true when signing validator manager tx outside of cli (for multisig or ledger)")
	cmd.Flags().StringVar(&validatorManagerOwner, "validator-manager-owner", "", "force using this address to issue transactions to the validator manager")
	cmd.Flags().StringVar(&initiateTxHash, "initiate-tx-hash", "", "initiate tx is already issued, with the given hash")
	cmd.SetHelpFunc(flags.WithGroupedHelp([]flags.GroupedFlags{sigAggGroup}))
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

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	if changeWeightFlags.RPC == "" {
		changeWeightFlags.RPC, _, err = contract.GetBlockchainEndpoints(
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

	validationID, err := validator.GetValidationID(changeWeightFlags.RPC, common.HexToAddress(validatorManagerAddress), nodeID)
	if err != nil {
		return err
	}
	if validationID == ids.Empty {
		return fmt.Errorf("node %s is not a L1 validator", nodeID)
	}

	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	var validatorInfo platformvm.L1Validator

	if initiateTxHash == "" {
		validatorInfo, err = validator.GetValidatorInfo(network.SDKNetwork(), validationID)
		if err != nil {
			return err
		}

		totalWeight, err := validator.GetTotalWeight(network.SDKNetwork(), subnetID)
		if err != nil {
			return err
		}

		allowedChange := float64(totalWeight) * constants.MaxL1TotalWeightChange
		allowedWeightFunction := func(v uint64) error {
			delta := uint64(0)
			if v > validatorInfo.Weight {
				delta = v - validatorInfo.Weight
			} else {
				delta = validatorInfo.Weight - v
			}
			if delta > uint64(allowedChange) {
				return fmt.Errorf("weight change %d exceeds max allowed weight change of %d", delta, uint64(allowedChange))
			}
			return nil
		}

		if !sc.UseACP99 {
			if float64(validatorInfo.Weight) > allowedChange {
				return fmt.Errorf("can't make change: current validator weight %d exceeds max allowed weight change of %d", validatorInfo.Weight, uint64(allowedChange))
			}
			allowedChange = float64(totalWeight-validatorInfo.Weight) * constants.MaxL1TotalWeightChange
			allowedWeightFunction = func(v uint64) error {
				if v > uint64(allowedChange) {
					return fmt.Errorf("new weight exceeds max allowed weight change of %d", uint64(allowedChange))
				}
				return nil
			}
		}

		if newWeight == 0 {
			ux.Logger.PrintToUser("Current validator weight is %d", validatorInfo.Weight)
			newWeight, err = app.Prompt.CaptureWeight(
				"What weight would you like to assign to the validator?",
				allowedWeightFunction,
			)
			if err != nil {
				return err
			}
		}

		if err := allowedWeightFunction(newWeight); err != nil {
			return err
		}
	}

	deployer := subnet.NewPublicDeployer(kc, network)

	if sc.UseACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: V2"))
		return changeWeightACP99(
			deployer,
			network,
			blockchainName,
			nodeID,
			newWeight,
			changeWeightFlags.SigAggFlags.SignatureAggregatorEndpoint,
			initiateTxHash,
		)
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: v1.0.0"))
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
		changeWeightFlags.RPC,
		changeWeightFlags.SigAggFlags.SignatureAggregatorEndpoint,
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
		changeWeightFlags.RPC,
		changeWeightFlags.SigAggFlags.SignatureAggregatorEndpoint,
	)
}

func changeWeightACP99(
	deployer *subnet.PublicDeployer,
	network models.Network,
	blockchainName string,
	nodeID ids.NodeID,
	weight uint64,
	signatureAggregatorEndpoint string,
	initiateTxHash string,
) error {
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}

	if validatorManagerOwner == "" {
		validatorManagerOwner = sc.ValidatorManagerOwner
	}

	var ownerPrivateKey string
	if !externalValidatorManagerOwner {
		var ownerPrivateKeyFound bool
		ownerPrivateKeyFound, _, _, ownerPrivateKey, err = contract.SearchForManagedKey(
			app,
			network,
			common.HexToAddress(validatorManagerOwner),
			true,
		)
		if err != nil {
			return err
		}
		if !ownerPrivateKeyFound {
			return fmt.Errorf("not private key found for Validator manager owner %s", validatorManagerOwner)
		}
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator manager owner %s pays for the initialization of the validator's weight change (Blockchain gas token)"), validatorManagerOwner)

	if sc.Networks[network.Name()].ValidatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}
	validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), changeWeightFlags.RPC)

	clusterName := sc.Networks[network.Name()].ClusterName
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		changeWeightFlags.SigAggFlags.AggregatorLogLevel,
		changeWeightFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return err
	}
	if signatureAggregatorEndpoint == "" {
		signatureAggregatorEndpoint, err = signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
		if err != nil {
			// if local machine does not have a running signature aggregator instance for the network, we will create it first
			err = signatureaggregator.CreateSignatureAggregatorInstance(app, network, aggregatorLogger, changeWeightFlags.SigAggFlags.SignatureAggregatorVersion)
			if err != nil {
				return err
			}
			signatureAggregatorEndpoint, err = signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
			if err != nil {
				return err
			}
		}
	}
	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	signedMessage, validationID, rawTx, err := validatormanager.InitValidatorWeightChange(
		aggregatorCtx,
		ux.Logger.PrintToUser,
		app,
		network,
		changeWeightFlags.RPC,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		nodeID,
		aggregatorLogger,
		validatorManagerAddress,
		weight,
		initiateTxHash,
		signatureAggregatorEndpoint,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Initializing Validator Weight Change", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}

	skipPChain := false
	if newWeight != 0 {
		// even if PChain already sent, validator should be available
		validatorInfo, err := validator.GetValidatorInfo(network.SDKNetwork(), validationID)
		if err != nil {
			return err
		}
		if validatorInfo.Weight == newWeight {
			ux.Logger.PrintToUser(logging.LightBlue.Wrap("The new Weight was already set on the P-Chain. Proceeding to the next step"))
			skipPChain = true
		}
	}
	if !skipPChain {
		txID, _, err := deployer.SetL1ValidatorWeight(signedMessage)
		if err != nil {
			if newWeight != 0 || !strings.Contains(err.Error(), "could not load L1 validator: not found") {
				return err
			}
			ux.Logger.PrintToUser(logging.LightBlue.Wrap("The Weight was already set to 0 on the P-Chain. Proceeding to the next step"))
		} else {
			ux.Logger.PrintToUser("SetL1ValidatorWeightTx ID: %s", txID)
			if err := blockchain.UpdatePChainHeight(
				"Waiting for P-Chain to update validator information ...",
			); err != nil {
				return err
			}
		}
	}

	aggregatorCtx, aggregatorCancel = sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	rawTx, err = validatormanager.FinishValidatorWeightChange(
		aggregatorCtx,
		app,
		network,
		changeWeightFlags.RPC,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		validationID,
		aggregatorLogger,
		validatorManagerAddress,
		signedMessage,
		newWeight,
		signatureAggregatorEndpoint,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Finish Validator Weight Change", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}

	ux.Logger.GreenCheckmarkToUser("Weight change successfully made")

	return nil
}

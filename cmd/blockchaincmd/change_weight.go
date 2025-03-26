// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	newWeight      uint64
	initiateTxHash string
)

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
	cmd.Flags().BoolVar(&externalValidatorManagerOwner, "external-evm-signature", false, "set this value to true when signing validator manager tx outside of cli (for multisig or ledger)")
	cmd.Flags().StringVar(&validatorManagerOwner, "validator-manager-owner", "", "force using this address to issue transactions to the validator manager")
	cmd.Flags().StringSliceVar(&aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().BoolVar(&aggregatorAllowPrivatePeers, "aggregator-allow-private-peers", true, "allow the signature aggregator to connect to peers with private IP")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&aggregatorLogToStdout, "aggregator-log-to-stdout", false, "use stdout for signature aggregator logs")
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "connect to validator manager at the given rpc endpoint")
	cmd.Flags().StringVar(&initiateTxHash, "initiate-tx-hash", "", "initiate tx is already issued, with the given hash")
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
	validationID, err := validator.GetValidationID(rpcURL, common.HexToAddress(validatorManagerAddress), nodeID)
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

	deployer := subnet.NewPublicDeployer(app, kc, network)

	if sc.UseACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: ACP99"))
		return changeWeightACP99(
			deployer,
			network,
			blockchainName,
			nodeID,
			newWeight,
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

func changeWeightACP99(
	deployer *subnet.PublicDeployer,
	network models.Network,
	blockchainName string,
	nodeID ids.NodeID,
	weight uint64,
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

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)

	clusterName := sc.Networks[network.Name()].ClusterName
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName, aggregatorExtraEndpoints)
	if err != nil {
		return err
	}
	aggregatorLogger, err := utils.NewLogger(
		constants.SignatureAggregatorLogName,
		aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		app.GetAggregatorLogDir(clusterName),
		aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
	if err != nil {
		return err
	}

	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()

	signedMessage, validationID, rawTx, err := validatormanager.InitValidatorWeightChange(
		aggregatorCtx,
		ux.Logger.PrintToUser,
		app,
		network,
		rpcURL,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		nodeID,
		extraAggregatorPeers,
		aggregatorAllowPrivatePeers,
		aggregatorLogger,
		validatorManagerAddress,
		weight,
		initiateTxHash,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		return evm.TxDump("Initializing Validator Weight Change", rawTx)
	}

	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	validatorInfo, err := validator.GetValidatorInfo(network.SDKNetwork(), validationID)
	if err != nil {
		return err
	}
	if validatorInfo.Weight == newWeight {
		ux.Logger.PrintToUser(logging.LightBlue.Wrap("The new Weight was already set on the P-Chain. Proceeding to the next step"))
	} else {
		txID, _, err := deployer.SetL1ValidatorWeight(signedMessage)
		if err != nil {
			if !strings.Contains(err.Error(), "could not load L1 validator: not found") {
				return err
			}
			ux.Logger.PrintToUser(logging.LightBlue.Wrap("The Validation ID was already removed on the P-Chain. Proceeding to the next step"))
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
		rpcURL,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		validationID,
		extraAggregatorPeers,
		aggregatorAllowPrivatePeers,
		aggregatorLogger,
		validatorManagerAddress,
		signedMessage,
		newWeight,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		return evm.TxDump("Finish Validator Weight Change", rawTx)
	}

	ux.Logger.GreenCheckmarkToUser("Weight change successfully made")

	return nil
}

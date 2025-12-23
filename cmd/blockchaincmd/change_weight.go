// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/duallogger"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validator"
	validatormanagersdk "github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/libevm/common"

	"github.com/spf13/cobra"
)

var (
	newWeight         uint64
	initiateTxHash    string
	changeWeightFlags BlockchainChangeWeightFlags
)

type BlockchainChangeWeightFlags struct {
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
	sigAggGroup := flags.AddSignatureAggregatorFlagsToCmd(cmd, &changeWeightFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().Uint64Var(&newWeight, "weight", 0, "set the new staking weight of the validator")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().StringVar(&pop, "bls-proof-of-possession", "", "set the BLS proof of possession of the validator")
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

	if sc.PoS() {
		return fmt.Errorf("weight can't be changed on Proof of Stake Validator Managers")
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

	validatorManagerRPCEndpoint := sc.Networks[network.Name()].ValidatorManagerRPCEndpoint
	validatorManagerAddress := sc.Networks[network.Name()].ValidatorManagerAddress
	specializedValidatorManagerAddress := sc.Networks[network.Name()].SpecializedValidatorManagerAddress
	if specializedValidatorManagerAddress != "" {
		validatorManagerAddress = specializedValidatorManagerAddress
	}

	if validatorManagerRPCEndpoint == "" {
		return fmt.Errorf("unable to find Validator Manager RPC endpoint")
	}
	if validatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}

	validationID, err := validatormanagersdk.GetValidationID(
		validatorManagerRPCEndpoint,
		common.HexToAddress(validatorManagerAddress),
		nodeID,
	)
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

		currentWeightInfo, err := validatormanagersdk.GetCurrentWeightInfo(
			network.SDKNetwork(),
			validatorManagerRPCEndpoint,
			common.HexToAddress(validatorManagerAddress),
		)
		if err != nil {
			return err
		}
		allowedChange := currentWeightInfo.AllowedWeight

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

	return changeWeight(
		deployer,
		network,
		blockchainName,
		nodeID,
		newWeight,
		changeWeightFlags.SigAggFlags.SignatureAggregatorEndpoint,
		initiateTxHash,
		kc,
	)
}

func changeWeight(
	deployer *subnet.PublicDeployer,
	network models.Network,
	blockchainName string,
	nodeID ids.NodeID,
	weight uint64,
	signatureAggregatorEndpoint string,
	initiateTxHash string,
	kc *keychain.Keychain,
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

	var signer *evm.Signer
	if externalValidatorManagerOwner {
		signer, err = evm.NewNoOpSigner(common.HexToAddress(validatorManagerOwner))
		if err != nil {
			return err
		}
	} else {
		ownerPrivateKeyFound, _, _, ownerPrivateKey, err := contract.SearchForManagedKey(
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
		signer, err = evm.NewSignerFromPrivateKey(ownerPrivateKey)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator manager owner %s pays for the initialization of the validator's weight change (Blockchain gas token)"), validatorManagerOwner)

	validatorManagerRPCEndpoint := sc.Networks[network.Name()].ValidatorManagerRPCEndpoint
	validatorManagerBlockchainID := sc.Networks[network.Name()].ValidatorManagerBlockchainID
	validatorManagerAddress := sc.Networks[network.Name()].ValidatorManagerAddress

	if validatorManagerRPCEndpoint == "" {
		return fmt.Errorf("unable to find Validator Manager RPC endpoint")
	}
	if validatorManagerBlockchainID == ids.Empty {
		return fmt.Errorf("unable to find Validator Manager blockchain ID")
	}
	if validatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), validatorManagerRPCEndpoint)

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
			err = signatureaggregator.CreateSignatureAggregatorInstance(app, network, aggregatorLogger, changeWeightFlags.SigAggFlags)
			if err != nil {
				return err
			}
			signatureAggregatorEndpoint, err = signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
			if err != nil {
				return err
			}
		}
	}

	// Get P-Chain's current epoch for SetL1ValidatorWeightMessage (signed by L1, verified by P-Chain)
	pChainEpoch, err := sdkutils.GetCurrentEpoch(network.Endpoint, "P")
	if err != nil {
		return fmt.Errorf("failure getting p-chain current epoch: %w", err)
	}
	epochTime := time.Unix(pChainEpoch.StartTime, 0)
	elapsed := time.Since(epochTime)
	if elapsed < constants.ProposerVMEpochDuration {
		time.Sleep(constants.ProposerVMEpochDuration - elapsed)
	}
	_, _, err = deployer.PChainTransfer(kc.Addresses().List()[0], 1)
	if err != nil {
		return fmt.Errorf("could not sent dummy transfer on p-chain: %w", err)
	}
	pChainEpoch, err = sdkutils.GetCurrentEpoch(network.Endpoint, "P")
	if err != nil {
		return fmt.Errorf("failure getting p-chain current epoch: %w", err)
	}

	ctx, cancel := sdkutils.GetTimedContext(constants.EVMEventLookupTimeout)
	defer cancel()
	signedMessage, validationID, rawTx, err := validatormanager.InitValidatorWeightChange(
		ctx,
		duallogger.NewDualLogger(true, app),
		app,
		network,
		validatorManagerRPCEndpoint,
		externalValidatorManagerOwner,
		signer,
		nodeID,
		aggregatorLogger,
		validatorManagerBlockchainID,
		validatorManagerAddress,
		weight,
		initiateTxHash,
		signatureAggregatorEndpoint,
		pChainEpoch.PChainHeight,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Initializing Validator Weight Change", rawTx)
		if err == nil {
			ux.Logger.PrintToUser("%s", dump)
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
			ux.Logger.PrintToUser("%s", logging.LightBlue.Wrap("The new Weight was already set on the P-Chain. Proceeding to the next step"))
			skipPChain = true
		}
	}
	if !skipPChain {
		txID, _, err := deployer.SetL1ValidatorWeight(signedMessage)
		if err != nil {
			if newWeight != 0 || !strings.Contains(err.Error(), "could not load L1 validator: not found") {
				return err
			}
			ux.Logger.PrintToUser("%s", logging.LightBlue.Wrap("The Weight was already set to 0 on the P-Chain. Proceeding to the next step"))
		} else {
			ux.Logger.PrintToUser("SetL1ValidatorWeightTx ID: %s", txID)
			if err := blockchain.UpdatePChainHeight("Waiting for P-Chain to update validator information ..."); err != nil {
				return err
			}
		}
	}

	// Get L1's current epoch for L1ValidatorWeightMessage (signed by P-Chain, verified by L1)
	l1Epoch, err := sdkutils.GetCurrentL1Epoch(validatorManagerRPCEndpoint, validatorManagerBlockchainID.String())
	if err != nil {
		return fmt.Errorf("failure getting l1 current epoch: %w", err)
	}
	epochTime = time.Unix(l1Epoch.StartTime, 0)
	elapsed = time.Since(epochTime)
	if elapsed < constants.ProposerVMEpochDuration {
		time.Sleep(constants.ProposerVMEpochDuration - elapsed)
	}
	client, err := evm.GetClient(validatorManagerRPCEndpoint)
	if err != nil {
		return fmt.Errorf("failure connecting to validator manager L1: %w", err)
	}
	if err := client.SetupProposerVM(signer); err != nil {
		return fmt.Errorf("failure setting proposer VM on L1: %w", err)
	}
	l1Epoch, err = sdkutils.GetCurrentL1Epoch(validatorManagerRPCEndpoint, validatorManagerBlockchainID.String())
	if err != nil {
		return fmt.Errorf("failure getting l1 current epoch: %w", err)
	}

	ctx, cancel = sdkutils.GetTimedContext(constants.EVMEventLookupTimeout)
	defer cancel()
	rawTx, err = validatormanager.FinishValidatorWeightChange(
		ctx,
		app.Log,
		app,
		network,
		validatorManagerRPCEndpoint,
		externalValidatorManagerOwner,
		signer,
		validationID,
		aggregatorLogger,
		validatorManagerBlockchainID,
		validatorManagerAddress,
		signedMessage,
		newWeight,
		signatureAggregatorEndpoint,
		l1Epoch.PChainHeight,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Finish Validator Weight Change", rawTx)
		if err == nil {
			ux.Logger.PrintToUser("%s", dump)
		}
		return err
	}

	ux.Logger.GreenCheckmarkToUser("Weight change successfully made")

	return nil
}

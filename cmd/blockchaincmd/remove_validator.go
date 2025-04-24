// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	validatorsdk "github.com/ava-labs/avalanche-cli/sdk/validator"
	validatormanagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	uptimeSec            uint64
	force                bool
	removeValidatorFlags BlockchainRemoveValidatorFlags
)

type BlockchainRemoveValidatorFlags struct {
	RPC         string
	SigAggFlags flags.SignatureAggregatorFlags
}

// avalanche blockchain removeValidator
func newRemoveValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "removeValidator [blockchainName]",
		Short: "Remove a permissioned validator from your blockchain",
		Long: `The blockchain removeValidator command stops a whitelisted blockchain network validator from
validating your deployed Blockchain.

To remove the validator from the Subnet's allow list, provide the validator's unique NodeID. You can bypass
these prompts by providing the values with flags.`,
		RunE: removeValidator,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.DefaultSupportedNetworkOptions)
	flags.AddRPCFlagToCmd(cmd, app, &removeValidatorFlags.RPC)
	flags.AddSignatureAggregatorFlagsToCmd(cmd, &removeValidatorFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji deploy only]")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "auth-keys", nil, "(for non-SOV blockchain only) control keys that will be used to authenticate the removeValidator tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "(for non-SOV blockchain only) file path of the removeValidator tx")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node-id of the validator")
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "remove validator that responds to the given endpoint")
	cmd.Flags().Uint64Var(&uptimeSec, "uptime", 0, "validator's uptime in seconds. If not provided, it will be automatically calculated")
	cmd.Flags().BoolVar(&force, "force", false, "force validator removal even if it's not getting rewarded")
	cmd.Flags().BoolVar(&externalValidatorManagerOwner, "external-evm-signature", false, "set this value to true when signing validator manager tx outside of cli (for multisig or ledger)")
	cmd.Flags().StringVar(&validatorManagerOwner, "validator-manager-owner", "", "force using this address to issue transactions to the validator manager")
	cmd.Flags().StringVar(&initiateTxHash, "initiate-tx-hash", "", "initiate tx is already issued, with the given hash")
	return cmd
}

func removeValidator(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	_, err := ValidateSubnetNameAndGetChains([]string{blockchainName})
	if err != nil {
		return err
	}

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
		networkoptions.GetNetworkFromSidecar(sc, networkoptions.DefaultSupportedNetworkOptions),
		"",
	)
	if err != nil {
		return err
	}
	if network.ClusterName != "" {
		network = models.ConvertClusterToNetwork(network)
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

	scNetwork := sc.Networks[network.Name()]
	subnetID := scNetwork.SubnetID
	if subnetID == ids.Empty {
		return constants.ErrNoSubnetID
	}

	var nodeID ids.NodeID
	switch {
	case nodeEndpoint != "":
		infoClient := info.NewClient(nodeEndpoint)
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		nodeID, _, err = infoClient.GetNodeID(ctx)
		if err != nil {
			return err
		}
	case nodeIDStr == "":
		nodeID, err = PromptNodeID("remove as a blockchain validator")
		if err != nil {
			return err
		}
	default:
		nodeID, err = ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
	}

	if sc.Sovereign && removeValidatorFlags.RPC == "" {
		removeValidatorFlags.RPC, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			contract.ChainSpec{
				BlockchainName: blockchainName,
			},
			true,
			false,
		)
		if err != nil {
			return err
		}
	}

	validatorKind, err := validatorsdk.IsSovereignValidator(network.SDKNetwork(), subnetID, nodeID)
	if err != nil {
		return err
	}
	if validatorKind == validatorsdk.NonValidator {
		// it may be unregistered from P-Chain, but registered on validator manager
		// due to a previous partial removal operation
		validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress
		validationID, err := validatorsdk.GetValidationID(
			removeValidatorFlags.RPC,
			common.HexToAddress(validatorManagerAddress),
			nodeID,
		)
		if err != nil {
			return err
		}
		if validationID != ids.Empty {
			validatorKind = validatorsdk.SovereignValidator
		}
	}
	if validatorKind == validatorsdk.NonValidator {
		return fmt.Errorf("node %s is not a validator of subnet %s on %s", nodeID, subnetID, network.Name())
	}

	if validatorKind == validatorsdk.SovereignValidator {
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

	deployer := subnet.NewPublicDeployer(app, kc, network)
	if validatorKind == validatorsdk.NonSovereignValidator {
		isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
		if err != nil {
			// just warn the user, don't fail
			ux.Logger.PrintToUser("failed to check if node is a validator on the subnet: %s", err)
		} else if !isValidator {
			// this is actually an error
			return fmt.Errorf("node %s is not a validator on subnet %s", nodeID, subnetID)
		}
		if err := UpdateKeychainWithSubnetControlKeys(kc, network, blockchainName); err != nil {
			return err
		}
		return removeValidatorNonSOV(deployer, network, subnetID, kc, blockchainName, nodeID)
	}
	if err := removeValidatorSOV(
		deployer,
		network,
		blockchainName,
		nodeID,
		uptimeSec,
		isBootstrapValidatorForNetwork(nodeID, scNetwork),
		force,
		removeValidatorFlags.RPC,
	); err != nil {
		return err
	}
	// remove the validator from the list of bootstrap validators
	newBootstrapValidators := utils.Filter(scNetwork.BootstrapValidators, func(b models.SubnetValidator) bool {
		if id, _ := ids.NodeIDFromString(b.NodeID); id != nodeID {
			return true
		}
		return false
	})
	// save new bootstrap validators and save sidecar
	scNetwork.BootstrapValidators = newBootstrapValidators
	sc.Networks[network.Name()] = scNetwork
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	return nil
}

func isBootstrapValidatorForNetwork(nodeID ids.NodeID, scNetwork models.NetworkData) bool {
	filteredBootstrapValidators := utils.Filter(scNetwork.BootstrapValidators, func(b models.SubnetValidator) bool {
		if id, err := ids.NodeIDFromString(b.NodeID); err == nil && id == nodeID {
			return true
		}
		return false
	})
	return len(filteredBootstrapValidators) > 0
}

func removeValidatorSOV(
	deployer *subnet.PublicDeployer,
	network models.Network,
	blockchainName string,
	nodeID ids.NodeID,
	uptimeSec uint64,
	isBootstrapValidator bool,
	force bool,
	rpcURL string,
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

	if sc.UseACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: ACP99"))
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: v1.0.0"))
	}

	ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator manager owner %s pays for the initialization of the validator's removal (Blockchain gas token)"), validatorManagerOwner)

	if sc.Networks[network.Name()].ValidatorManagerAddress == "" {
		return fmt.Errorf("unable to find Validator Manager address")
	}
	validatorManagerAddress = sc.Networks[network.Name()].ValidatorManagerAddress

	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)

	clusterName := sc.Networks[network.Name()].ClusterName
	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return err
	}
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLoggerNewLogger(
		removeValidatorFlags.SigAggFlags.AggregatorLogLevel,
		removeValidatorFlags.SigAggFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return err
	}
	if force && sc.PoS() {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Forcing removal of %s as it is a PoS bootstrap validator"), nodeID)
	}

	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()

	// try to remove the validator. If err is "delegator ineligible for rewards" confirm with user and force remove
	signedMessage, validationID, rawTx, err := validatormanager.InitValidatorRemoval(
		aggregatorCtx,
		app,
		network,
		rpcURL,
		chainSpec,
		externalValidatorManagerOwner,
		validatorManagerOwner,
		ownerPrivateKey,
		nodeID,
		extraAggregatorPeers,
		aggregatorLogger,
		sc.PoS(),
		uptimeSec,
		isBootstrapValidator || force,
		validatorManagerAddress,
		sc.UseACP99,
		initiateTxHash,
	)
	if err != nil && errors.Is(err, validatormanagerSDK.ErrValidatorIneligibleForRewards) {
		ux.Logger.PrintToUser("Calculated rewards is zero. Validator %s is not eligible for rewards", nodeID)
		force, err = app.Prompt.CaptureNoYes("Do you want to continue with validator removal?")
		if err != nil {
			return err
		}
		if !force {
			return fmt.Errorf("validator %s is not eligible for rewards. Use --force flag to force removal", nodeID)
		}
		aggregatorCtx, aggregatorCancel = sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
		defer aggregatorCancel()
		signedMessage, validationID, _, err = validatormanager.InitValidatorRemoval(
			aggregatorCtx,
			app,
			network,
			rpcURL,
			chainSpec,
			externalValidatorManagerOwner,
			validatorManagerOwner,
			ownerPrivateKey,
			nodeID,
			extraAggregatorPeers,
			aggregatorLogger,
			sc.PoS(),
			uptimeSec,
			true, // force
			validatorManagerAddress,
			sc.UseACP99,
			initiateTxHash,
		)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Initializing Validator Removal", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}

	ux.Logger.PrintToUser("ValidationID: %s", validationID)
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

	aggregatorCtx, aggregatorCancel = sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	rawTx, err = validatormanager.FinishValidatorRemoval(
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
		aggregatorLogger,
		validatorManagerAddress,
		sc.PoA() && sc.UseACP99,
	)
	if err != nil {
		return err
	}
	if rawTx != nil {
		dump, err := evm.TxDump("Finish Validator Removal", rawTx)
		if err == nil {
			ux.Logger.PrintToUser(dump)
		}
		return err
	}

	ux.Logger.GreenCheckmarkToUser("Validator successfully removed from the Subnet")

	return nil
}

func removeValidatorNonSOV(deployer *subnet.PublicDeployer, network models.Network, subnetID ids.ID, kc *keychain.Keychain, blockchainName string, nodeID ids.NodeID) error {
	_, controlKeys, threshold, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
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
	ux.Logger.PrintToUser("Your auth keys for remove validator tx creation: %s", subnetAuthKeys)

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

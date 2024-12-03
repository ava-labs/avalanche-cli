// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
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
	networkoptions.EtnaDevnet,
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
	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "remove validator that responds to the given endpoint")
	cmd.Flags().StringSliceVar(&aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	privateKeyFlags.AddToCmd(cmd, "to pay fees for completing the validator's removal (blockchain gas token)")
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "connect to validator manager at the given rpc endpoint")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")
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
	if network.ClusterName != "" {
		network = models.ConvertClusterToNetwork(network)
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

	if !sc.Sovereign {
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

	if network.Kind == models.Local && !sc.Sovereign {
		return removeFromLocalNonSOV(blockchainName, nodeID)
	}

	scNetwork := sc.Networks[network.Name()]
	subnetID := scNetwork.SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)
	// check that this guy actually is a validator on the subnet
	if !sc.Sovereign {
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
	// check if node is a bootstrap validator to force it to be removed
	filteredBootstrapValidators := utils.Filter(scNetwork.BootstrapValidators, func(b models.SubnetValidator) bool {
		if id, err := ids.NodeIDFromString(b.NodeID); err == nil && id == nodeID {
			return true
		}
		return false
	})
	force := len(filteredBootstrapValidators) > 0
	if err := removeValidatorSOV(deployer, network, blockchainName, nodeID, force); err != nil {
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

func removeValidatorSOV(
	deployer *subnet.PublicDeployer,
	network models.Network,
	blockchainName string,
	nodeID ids.NodeID,
	force bool,
) error {
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	ownerPrivateKeyFound, _, _, ownerPrivateKey, err := contract.SearchForManagedKey(
		app,
		network,
		common.HexToAddress(sc.ValidatorManagerOwner),
		true,
	)
	if err != nil {
		return err
	}
	if !ownerPrivateKeyFound {
		return fmt.Errorf("not private key found for Validator manager owner %s", sc.ValidatorManagerOwner)
	}
	ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator manager owner %s pays for the initialization of the validator's removal (Blockchain gas token)"), sc.ValidatorManagerOwner)

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
	ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)
	clusterName := sc.Networks[network.Name()].ClusterName
	extraAggregatorPeers, err := GetAggregatorExtraPeers(clusterName, aggregatorExtraEndpoints)
	if err != nil {
		return err
	}
	if force && sc.PoS() {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Forcing removal of %s as it is a PoS bootstrap validator"), nodeID)
	}

	signedMessage, validationID, err := validatormanager.InitValidatorRemoval(
		app,
		network,
		rpcURL,
		chainSpec,
		ownerPrivateKey,
		nodeID,
		extraAggregatorPeers,
		aggregatorLogLevel,
		sc.PoS(),
		force,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("ValidationID: %s", validationID)

	txID, _, err := deployer.SetL1ValidatorWeight(signedMessage)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("SetL1ValidatorWeightTx ID: %s", txID)

	if err := validatormanager.FinishValidatorRemoval(
		app,
		network,
		rpcURL,
		chainSpec,
		ownerPrivateKey,
		validationID,
		extraAggregatorPeers,
		aggregatorLogLevel,
	); err != nil {
		return err
	}

	ux.Logger.GreenCheckmarkToUser("Validator successfully removed from the Subnet")

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

func removeFromLocalNonSOV(
	blockchainName string,
	nodeID ids.NodeID,
) error {
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

	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	_, err = subnet.IssueRemoveSubnetValidatorTx(keyChain, subnetID, nodeID)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Validator removed")

	return nil
}

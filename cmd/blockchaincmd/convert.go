// / Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	validatormanagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	doStrongInputChecks bool
	convertFlags        BlockchainConvertFlags
)

type BlockchainConvertFlags struct {
	SigAggFlags             flags.SignatureAggregatorFlags
	LocalMachineFlags       flags.LocalMachineFlags
	ProofOfStakeFlags       flags.POSFlags
	BootstrapValidatorFlags flags.BootstrapValidatorFlags
	ConvertOnly             bool
}

// avalanche blockchain convert
func newConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [blockchainName]",
		Short: "Converts a Subnet into a sovereign L1",
		Long: `The blockchain convert command converts a Subnet into sovereign L1.

Sovereign L1s require bootstrap validators. avalanche blockchain convert command gives the option of: 
- either using local machine as bootstrap validators (set the number of bootstrap validators using 
--num-bootstrap-validators flag, default is set to 1)
- or using remote nodes (we require the node's Node-ID and BLS info)`,
		RunE:              convertBlockchain,
		PersistentPostRun: handlePostRun,
		PreRunE:           cobrautils.ExactArgs(1),
	}
	networkGroup := networkoptions.GetNetworkFlagsGroup(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	sigAggGroup := flags.AddSignatureAggregatorFlagsToCmd(cmd, &convertFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use to authorize ConvertSubnetTol1 Tx")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "auth-keys", nil, "control keys that will be used to authenticate ConvertSubnetTol1")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the convert to L1 tx (for multi-sig)")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().BoolVar(&convertFlags.ConvertOnly, "convert-only", false, "avoid node track, restart and poa manager setup")

	localMachineGroup := flags.AddLocalMachineFlagsToCmd(cmd, &convertFlags.LocalMachineFlags)
	posGroup := flags.AddProofOfStakeToCmd(cmd, &convertFlags.ProofOfStakeFlags)
	bootstrapValidatorGroup := flags.AddBootstrapValidatorFlagsToCmd(cmd, &convertFlags.BootstrapValidatorFlags)

	cmd.Flags().BoolVar(&createFlags.proofOfAuthority, "proof-of-authority", false, "use proof of authority(PoA) for validator management")
	cmd.Flags().BoolVar(&createFlags.proofOfStake, "proof-of-stake", false, "use proof of stake(PoS) for validator management")
	cmd.Flags().StringVar(&createFlags.validatorManagerOwner, "validator-manager-owner", "", "EVM address that controls Validator Manager Owner")
	cmd.Flags().StringVar(&validatorManagerRPCEndpoint, "validator-manager-rpc", "", "RPC to use to access to the validator manager")
	cmd.Flags().StringVar(&validatorManagerBlockchainIDStr, "validator-manager-blockchain-id", "", "validator manager blockchain ID")
	cmd.Flags().StringVar(&validatorManagerAddressStr, "validator-manager-address", "", "validator manager address")
	cmd.Flags().BoolVar(&doStrongInputChecks, "verify-input", true, "check for input confirmation")
	cmd.SetHelpFunc(flags.WithGroupedHelp([]flags.GroupedFlags{networkGroup, bootstrapValidatorGroup, localMachineGroup, posGroup, sigAggGroup}))
	return cmd
}

func GetValidatorManagerRPCEndpoint(
	network models.Network,
	blockchainName string,
	blockchainID ids.ID,
	validatorManagerBlockchainID ids.ID,
) (string, error) {
	var (
		err                         error
		validatorManagerRPCEndpoint string
	)
	if blockchainID == validatorManagerBlockchainID {
		validatorManagerRPCEndpoint, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			contract.ChainSpec{
				BlockchainName: blockchainName,
				BlockchainID:   blockchainID.String(),
			},
			true,
			false,
		)
		if err != nil {
			return "", err
		}
	} else {
		cChainID, err := contract.GetBlockchainID(app, network, contract.ChainSpec{CChain: true})
		if err != nil {
			return "", fmt.Errorf("could not get C-Chain ID for %s: %w", network.Name(), err)
		}
		if cChainID == validatorManagerBlockchainID {
			// manager lives at C-Chain
			validatorManagerRPCEndpoint = network.CChainEndpoint()
		} else {
			validatorManagerRPCEndpoint, err = app.Prompt.CaptureURL("What is the Validator Manager RPC endpoint?", false)
			if err != nil {
				return "", err
			}
		}
	}
	return validatorManagerRPCEndpoint, nil
}

func InitializeValidatorManager(
	blockchainName string,
	subnetID ids.ID,
	blockchainID ids.ID,
	network models.Network,
	avaGoBootstrapValidators []*txs.ConvertSubnetToL1Validator,
	pos bool,
	validatorManagerRPCEndpoint string,
	validatorManagerBlockchainID ids.ID,
	validatorManagerAddressStr string,
	validatorManagerOwnerAddressStr string,
	useACP99 bool,
	useLocalMachine bool,
	signatureAggregatorFlags flags.SignatureAggregatorFlags,
	proofOfStakeFlags flags.POSFlags,
) (bool, error) {
	if useACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: V2"))
	} else {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: v1.0.0"))
	}

	var err error
	clusterName := clusterNameFlagValue
	switch {
	case useLocalMachine:
		if err := localnet.LocalClusterTrackSubnet(
			app,
			ux.Logger.PrintToUser,
			clusterName,
			blockchainName,
		); err != nil {
			return false, err
		}
	default:
		if clusterName != "" {
			if err = node.SyncSubnet(app, clusterName, blockchainName, true, nil); err != nil {
				return false, err
			}

			if err := node.WaitForHealthyCluster(app, clusterName, node.HealthCheckTimeout, node.HealthCheckPoolTime); err != nil {
				return false, err
			}
		}
	}

	tracked := true

	if blockchainID == validatorManagerBlockchainID && validatorManagerAddressStr == validatormanagerSDK.ValidatorProxyContractAddress {
		// we assume it is fully CLI managed
		if err := CompleteValidatorManagerL1Deploy(
			network,
			blockchainName,
			validatorManagerRPCEndpoint,
			validatorManagerOwnerAddressStr,
			pos,
			useACP99,
		); err != nil {
			return tracked, err
		}
	}

	if validatorManagerRPCEndpoint == "" {
		validatorManagerRPCEndpoint, err = GetValidatorManagerRPCEndpoint(
			network,
			blockchainName,
			blockchainID,
			validatorManagerBlockchainID,
		)
		if err != nil {
			return tracked, err
		}
	}

	client, err := evm.GetClient(validatorManagerRPCEndpoint)
	if err != nil {
		return tracked, err
	}
	if err := client.WaitForEVMBootstrapped(0); err != nil {
		return tracked, err
	}

	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return tracked, err
	}

	validatorManagerAddress := common.HexToAddress(validatorManagerAddressStr)

	_, _, specializedValidatorManagerAddress, err := GetBaseValidatorManagerInfo(
		validatorManagerRPCEndpoint,
		validatorManagerAddress,
	)
	if err != nil {
		return tracked, err
	}

	validatorManagerOwnerAddress := common.HexToAddress(validatorManagerOwnerAddressStr)

	found, _, _, validatorManagerOwnerPrivateKey, err := contract.SearchForManagedKey(
		app,
		network,
		validatorManagerOwnerAddress,
		true,
	)
	if err != nil {
		return tracked, err
	}
	if !found {
		return tracked, fmt.Errorf("could not find validator manager owner private key")
	}

	subnetSDK := blockchainSDK.Subnet{
		Network:                            network.SDKNetwork(),
		SubnetID:                           subnetID,
		ValidatorManagerRPC:                validatorManagerRPCEndpoint,
		ValidatorManagerBlockchainID:       validatorManagerBlockchainID,
		ValidatorManagerAddress:            &validatorManagerAddress,
		SpecializedValidatorManagerAddress: &specializedValidatorManagerAddress,
		ValidatorManagerOwnerAddress:       &validatorManagerOwnerAddress,
		ValidatorManagerOwnerPrivateKey:    validatorManagerOwnerPrivateKey,
		BootstrapValidators:                avaGoBootstrapValidators,
	}
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLogger(
		signatureAggregatorFlags.AggregatorLogLevel,
		signatureAggregatorFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return tracked, err
	}
	// TODO: replace latest below with sig agg version in flags for convert and deploy
	err = signatureaggregator.CreateSignatureAggregatorInstance(app, subnetID.String(), network, extraAggregatorPeers, aggregatorLogger, "latest")
	if err != nil {
		return tracked, err
	}
	signatureAggregatorEndpoint, err := signatureaggregator.GetSignatureAggregatorEndpoint(app, network)
	if err != nil {
		return tracked, err
	}
	if pos {
		ux.Logger.PrintToUser("Initializing Native Token Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
		if err := subnetSDK.InitializeProofOfStake(
			app.Log,
			validatorManagerOwnerPrivateKey,
			aggregatorLogger,
			validatormanagerSDK.PoSParams{
				MinimumStakeAmount:      big.NewInt(int64(proofOfStakeFlags.MinimumStakeAmount)),
				MaximumStakeAmount:      big.NewInt(int64(proofOfStakeFlags.MaximumStakeAmount)),
				MinimumStakeDuration:    proofOfStakeFlags.MinimumStakeDuration,
				MinimumDelegationFee:    proofOfStakeFlags.MinimumDelegationFee,
				MaximumStakeMultiplier:  proofOfStakeFlags.MaximumStakeMultiplier,
				WeightToValueFactor:     big.NewInt(int64(proofOfStakeFlags.WeightToValueFactor)),
				RewardCalculatorAddress: validatormanagerSDK.RewardCalculatorAddress,
				UptimeBlockchainID:      blockchainID,
			},
			useACP99,
			signatureAggregatorEndpoint,
		); err != nil {
			return tracked, err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	} else {
		ux.Logger.PrintToUser("Initializing Proof of Authority Validator Manager contract on blockchain %s ...", blockchainName)
		if err := subnetSDK.InitializeProofOfAuthority(
			app.Log,
			validatorManagerOwnerPrivateKey,
			aggregatorLogger,
			useACP99,
			signatureAggregatorEndpoint,
		); err != nil {
			return tracked, err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	}

	sidecar, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return tracked, err
	}
	if specializedValidatorManagerAddress != (common.Address{}) {
		validatorManagerAddress = specializedValidatorManagerAddress
	}
	sidecar.UpdateValidatorManagerAddress(network.Name(), validatorManagerRPCEndpoint, validatorManagerBlockchainID, validatorManagerAddress.String())
	if err := app.UpdateSidecar(&sidecar); err != nil {
		return tracked, err
	}

	return tracked, nil
}

func convertSubnetToL1(
	bootstrapValidators []models.SubnetValidator,
	deployer *subnet.PublicDeployer,
	subnetID ids.ID,
	blockchainID ids.ID,
	network models.Network,
	chain string,
	sidecar models.Sidecar,
	controlKeysList,
	subnetAuthKeysList []string,
	validatorManagerBlockchainID ids.ID,
	validatorManagerAddress string,
	doStrongInputsCheck bool,
) ([]*txs.ConvertSubnetToL1Validator, bool, bool, error) {
	if subnetID == ids.Empty {
		return nil, false, false, constants.ErrNoSubnetID
	}
	if blockchainID == ids.Empty {
		return nil, false, false, constants.ErrNoBlockchainID
	}
	if validatorManagerBlockchainID == ids.Empty {
		return nil, false, false, constants.ErrNoBlockchainID
	}
	if !common.IsHexAddress(validatorManagerAddress) {
		return nil, false, false, constants.ErrInvalidValidatorManagerAddress
	}
	avaGoBootstrapValidators, err := ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return avaGoBootstrapValidators, false, false, err
	}

	if doStrongInputsCheck {
		ux.Logger.PrintToUser("You are about to create a ConvertSubnetToL1Tx on %s with the following content:", network.Name())
		ux.Logger.PrintToUser("  Subnet ID: %s", subnetID)
		ux.Logger.PrintToUser("  Manager Blockchain ID: %s", validatorManagerBlockchainID)
		ux.Logger.PrintToUser("  Manager Address: %s", validatorManagerAddress)
		ux.Logger.PrintToUser("  Validators:")
		for _, val := range bootstrapValidators {
			ux.Logger.PrintToUser("    Node ID: %s", val.NodeID)
			ux.Logger.PrintToUser("    Weight: %d", val.Weight)
			ux.Logger.PrintToUser("    Balance: %.5f", float64(val.Balance)/float64(units.Avax))
		}
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please review the details of the ConvertSubnetToL1 Transaction")
		ux.Logger.PrintToUser("")
		if doContinue, err := app.Prompt.CaptureYesNo("Do you want to create the transaction?"); err != nil {
			return avaGoBootstrapValidators, false, false, err
		} else if !doContinue {
			return avaGoBootstrapValidators, true, false, nil
		}
	}

	isFullySigned, convertL1TxID, tx, remainingSubnetAuthKeys, err := deployer.ConvertL1(
		controlKeysList,
		subnetAuthKeysList,
		subnetID,
		validatorManagerBlockchainID,
		common.HexToAddress(validatorManagerAddress),
		avaGoBootstrapValidators,
	)
	if err != nil {
		ux.Logger.RedXToUser("error converting blockchain: %s. fix the issue and try again with a new convert cmd", err)
		return avaGoBootstrapValidators, false, false, err
	}

	savePartialTx := !isFullySigned && err == nil

	if savePartialTx {
		if err := SaveNotFullySignedTx(
			"ConvertSubnetToL1Tx",
			tx,
			chain,
			subnetAuthKeys,
			remainingSubnetAuthKeys,
			outputTxPath,
			false,
		); err != nil {
			return avaGoBootstrapValidators, false, savePartialTx, err
		}
	} else {
		ux.Logger.PrintToUser("ConvertSubnetToL1Tx ID: %s", convertL1TxID)
		_, err = ux.TimedProgressBar(
			30*time.Second,
			"Waiting for the Subnet to be converted into a sovereign L1 ...",
			0,
		)
		if err != nil {
			return avaGoBootstrapValidators, false, savePartialTx, err
		}
	}

	ux.Logger.PrintToUser("")
	setBootstrapValidatorValidationID(avaGoBootstrapValidators, bootstrapValidators, subnetID)
	return avaGoBootstrapValidators, false, savePartialTx, app.UpdateSidecarNetworks(
		&sidecar,
		network,
		subnetID,
		blockchainID,
		"",
		"",
		bootstrapValidators,
		clusterNameFlagValue,
		"",
		validatorManagerBlockchainID,
		validatorManagerAddress,
	)
}

// convertBlockchain is the cobra command run for converting subnets into sovereign L1
func convertBlockchain(cmd *cobra.Command, args []string) error {
	blockchainName := args[0]

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	var bootstrapValidators []models.SubnetValidator
	if convertFlags.BootstrapValidatorFlags.BootstrapValidatorsJSONFilePath != "" {
		bootstrapValidators, err = LoadBootstrapValidator(convertFlags.BootstrapValidatorFlags)
		if err != nil {
			return err
		}
		convertFlags.BootstrapValidatorFlags.NumBootstrapValidators = len(bootstrapValidators)
	}

	chain := chains[0]

	sidecar, err := app.LoadSidecar(chain)
	if err != nil {
		return fmt.Errorf("failed to load sidecar for later update: %w", err)
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkoptions.GetNetworkFromSidecar(sidecar, networkoptions.DefaultSupportedNetworkOptions),
		"",
	)
	if err != nil {
		return err
	}

	if err = validateConvertOnlyFlag(cmd, convertFlags.BootstrapValidatorFlags, &convertFlags.ConvertOnly, convertFlags.LocalMachineFlags.UseLocalMachine); err != nil {
		return err
	}

	clusterNameFlagValue = globalNetworkFlags.ClusterName

	subnetID := sidecar.Networks[network.Name()].SubnetID
	blockchainID := sidecar.Networks[network.Name()].BlockchainID

	if doStrongInputChecks && subnetID != ids.Empty {
		ux.Logger.PrintToUser("Subnet ID to be used is %s", subnetID)
		if acceptValue, err := app.Prompt.CaptureYesNo("Is this value correct?"); err != nil {
			return err
		} else if !acceptValue {
			subnetID = ids.Empty
		}
	}
	if subnetID == ids.Empty {
		subnetID, err = app.Prompt.CaptureID("What is the subnet ID?")
		if err != nil {
			return err
		}
	}

	if doStrongInputChecks && blockchainID != ids.Empty {
		ux.Logger.PrintToUser("L1 Blockchain ID to be used is %s", blockchainID)
		if acceptValue, err := app.Prompt.CaptureYesNo("Is this value correct?"); err != nil {
			return err
		} else if !acceptValue {
			blockchainID = ids.Empty
		}
	}
	if blockchainID == ids.Empty {
		blockchainID, err = app.Prompt.CaptureID("What is the L1 blockchain ID?")
		if err != nil {
			return err
		}
	}

	var validatorManagerBlockchainID ids.ID
	if validatorManagerBlockchainIDStr != "" {
		validatorManagerBlockchainID, err = ids.FromString(validatorManagerBlockchainIDStr)
		if err != nil {
			return err
		}
	} else {
		// if not given, assume for the moment it is the same L1
		validatorManagerBlockchainID = blockchainID
	}
	if doStrongInputChecks && validatorManagerBlockchainID != ids.Empty {
		ux.Logger.PrintToUser("Validator Manager Blockchain ID to be used is %s", validatorManagerBlockchainID)
		if acceptValue, err := app.Prompt.CaptureYesNo("Is this value correct?"); err != nil {
			return err
		} else if !acceptValue {
			validatorManagerBlockchainID = ids.Empty
		}
	}
	if validatorManagerBlockchainID == ids.Empty {
		validatorManagerBlockchainID, err = app.Prompt.CaptureID("What is the Validator Manager blockchain ID?")
		if err != nil {
			return err
		}
	}

	if validatorManagerAddressStr == "" {
		validatorManagerAddress, err := app.Prompt.CaptureAddress("What is the address of the Validator Manager?")
		if err != nil {
			return err
		}
		validatorManagerAddressStr = validatorManagerAddress.String()
	}

	if !convertFlags.ConvertOnly {
		if err = promptValidatorManagementType(app, &sidecar); err != nil {
			return err
		}
		if err := setSidecarValidatorManageOwner(&sidecar, createFlags); err != nil {
			return err
		}
		sidecar.UpdateValidatorManagerAddress(network.Name(), "", validatorManagerBlockchainID, validatorManagerAddressStr)
	}

	sidecar.Sovereign = true
	fee := uint64(0)

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

	availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
	if err != nil {
		return err
	}

	deployBalance := uint64(convertFlags.BootstrapValidatorFlags.DeployBalanceAVAX * float64(units.Avax))

	err = prepareBootstrapValidators(
		&bootstrapValidators,
		network,
		sidecar,
		*kc,
		blockchainName,
		deployBalance,
		availableBalance,
		&convertFlags.LocalMachineFlags,
		&convertFlags.BootstrapValidatorFlags,
	)
	if err != nil {
		return err
	}

	requiredBalance := deployBalance * uint64(len(bootstrapValidators))
	if availableBalance < requiredBalance {
		return fmt.Errorf(
			"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
			len(bootstrapValidators),
			requiredBalance,
			availableBalance,
		)
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for blockchain tx signing
	_, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	// get keys for convertL1 tx signing
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
	ux.Logger.PrintToUser("Your auth keys for add validator tx creation: %s", subnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, kc, network)

	avaGoBootstrapValidators, cancel, savePartialTx, err := convertSubnetToL1(
		bootstrapValidators,
		deployer,
		subnetID,
		blockchainID,
		network,
		chain,
		sidecar,
		controlKeys,
		subnetAuthKeys,
		validatorManagerBlockchainID,
		validatorManagerAddressStr,
		doStrongInputChecks,
	)
	if err != nil {
		return err
	}
	if cancel {
		return nil
	}

	if savePartialTx {
		return nil
	}

	if !convertFlags.ConvertOnly && !convertFlags.BootstrapValidatorFlags.GenerateNodeID {
		if _, err = InitializeValidatorManager(
			blockchainName,
			subnetID,
			blockchainID,
			network,
			avaGoBootstrapValidators,
			sidecar.ValidatorManagement == validatormanagertypes.ProofOfStake,
			validatorManagerRPCEndpoint,
			validatorManagerBlockchainID,
			validatorManagerAddressStr,
			sidecar.ValidatorManagerOwner,
			sidecar.UseACP99,
			convertFlags.LocalMachineFlags.UseLocalMachine,
			convertFlags.SigAggFlags,
			convertFlags.ProofOfStakeFlags,
		); err != nil {
			return err
		}
	} else {
		printSuccessfulConvertOnlyOutput(blockchainName, subnetID.String(), convertFlags.BootstrapValidatorFlags.GenerateNodeID)
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Your L1 is ready for on-chain interactions."))
	ux.Logger.PrintToUser("")
	ux.Logger.GreenCheckmarkToUser("Subnet is successfully converted to sovereign L1")

	return nil
}

func printSuccessfulConvertOnlyOutput(blockchainName, subnetID string, generateNodeID bool) {
	ux.Logger.GreenCheckmarkToUser("Converted blockchain successfully generated")
	ux.Logger.PrintToUser("Next, we need to:")
	if generateNodeID {
		ux.Logger.PrintToUser("- Create the corresponding Avalanche node(s) with the provided Node ID and BLS Info")
	}
	ux.Logger.PrintToUser("- Have the Avalanche node(s) track the blockchain")
	ux.Logger.PrintToUser("- Call `avalanche contract initValidatorManager %s`", blockchainName)
	ux.Logger.PrintToUser("==================================================")
	if generateNodeID {
		ux.Logger.PrintToUser("To create the Avalanche node(s) with the provided Node ID and BLS Info:")
		ux.Logger.PrintToUser("- Created Node ID and BLS Info can be found at %s", app.GetSidecarPath(blockchainName))
		ux.Logger.PrintToUser("")
	}
	ux.Logger.PrintToUser("To enable the nodes to track the L1:")
	ux.Logger.PrintToUser("- Set '%s' as the value for 'track-subnets' configuration in ~/.avalanchego/config.json", subnetID)
	ux.Logger.PrintToUser("- Ensure that the P2P port is exposed and 'public-ip' config value is set")
}

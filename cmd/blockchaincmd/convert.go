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
	"github.com/ava-labs/avalanche-cli/pkg/dependencies"
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
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	validatormanagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/config"
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
	cmd.Flags().StringVar(&createFlags.proxyContractOwner, "proxy-contract-owner", "", "EVM address that controls ProxyAdmin for TransparentProxy of ValidatorManager contract")
	cmd.Flags().StringVar(&validatorManagerAddress, "validator-manager-address", "", "validator manager address")
	cmd.Flags().BoolVar(&doStrongInputChecks, "verify-input", true, "check for input confirmation")
	cmd.SetHelpFunc(flags.WithGroupedHelp([]flags.GroupedFlags{networkGroup, bootstrapValidatorGroup, localMachineGroup, posGroup, sigAggGroup}))
	return cmd
}

func StartLocalMachine(
	network models.Network,
	sidecar models.Sidecar,
	blockchainName string,
	deployBalance,
	availableBalance uint64,
	localMachineFlags *flags.LocalMachineFlags,
	bootstrapValidatorFlags *flags.BootstrapValidatorFlags,
) (bool, error) {
	var err error
	if network.Kind == models.Local &&
		!bootstrapValidatorFlags.GenerateNodeID &&
		bootstrapValidatorFlags.BootstrapEndpoints == nil &&
		bootstrapValidatorFlags.BootstrapValidatorsJSONFilePath == "" {
		localMachineFlags.UseLocalMachine = true
	}
	clusterName := localnet.LocalClusterName(network, blockchainName)
	if clusterNameFlagValue != "" {
		clusterName = clusterNameFlagValue
		if localnet.LocalClusterExists(app, clusterName) {
			localMachineFlags.UseLocalMachine = true
			if len(bootstrapValidatorFlags.BootstrapEndpoints) == 0 {
				bootstrapValidatorFlags.BootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
				if err != nil {
					return false, fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
						"please create your local node again and call blockchain deploy command again", err)
				}
			}
			network = models.ConvertClusterToNetwork(network)
		}
	}
	// ask user if we want to use local machine if cluster is not provided
	if !localMachineFlags.UseLocalMachine && clusterNameFlagValue == "" {
		ux.Logger.PrintToUser("You can use your local machine as a bootstrap validator on the blockchain")
		ux.Logger.PrintToUser("This means that you don't have to to set up a remote server on a cloud service (e.g. AWS / GCP) to be a validator on the blockchain.")

		localMachineFlags.UseLocalMachine, err = app.Prompt.CaptureYesNo("Do you want to use your local machine as a bootstrap validator?")
		if err != nil {
			return false, err
		}
	}
	// default number of local machine nodes to be 1
	if localMachineFlags.UseLocalMachine && bootstrapValidatorFlags.NumBootstrapValidators == 0 {
		bootstrapValidatorFlags.NumBootstrapValidators = constants.DefaultNumberOfLocalMachineNodes
	}
	// if no cluster provided - we create one with fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent) name
	if localMachineFlags.UseLocalMachine && clusterNameFlagValue == "" {
		if localnet.LocalClusterExists(app, clusterName) {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser(
				logging.Red.Wrap("A local machine L1 deploy already exists for %s L1 and network %s"),
				blockchainName,
				network.Name(),
			)
			yes, err := app.Prompt.CaptureNoYes(
				fmt.Sprintf("Do you want to overwrite the current local L1 deploy for %s?", blockchainName),
			)
			if err != nil {
				return false, err
			}
			if !yes {
				return true, nil
			}
			_ = localnet.LocalClusterRemove(app, clusterName)
			ux.Logger.GreenCheckmarkToUser("Local node %s cleaned up.", clusterName)
		}
		requiredBalance := deployBalance * uint64(bootstrapValidatorFlags.NumBootstrapValidators)
		if availableBalance < requiredBalance {
			return false, fmt.Errorf(
				"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
				bootstrapValidatorFlags.NumBootstrapValidators,
				requiredBalance,
				availableBalance,
			)
		}
		avagoVersionSettings := dependencies.AvalancheGoVersionSettings{}
		// setup (install if needed) avalanchego binary
		avagoVersion := localMachineFlags.UserProvidedAvagoVersion
		if localMachineFlags.UserProvidedAvagoVersion == constants.DefaultAvalancheGoVersion && localMachineFlags.AvagoBinaryPath == "" {
			// nothing given: get avago version from RPC compat using latest.json defined in
			// https://raw.githubusercontent.com/ava-labs/avalanche-cli/control-default-version/versions/latest.json
			avagoVersion, err = dependencies.GetLatestCLISupportedDependencyVersion(app, constants.AvalancheGoRepoName, network, &sidecar.RPCVersion)
			if err != nil {
				if err != dependencies.ErrNoAvagoVersion {
					return false, err
				}
				avagoVersion = constants.LatestPreReleaseVersionTag
			}
		}
		localMachineFlags.AvagoBinaryPath, err = localnet.SetupAvalancheGoBinary(app, avagoVersion, localMachineFlags.AvagoBinaryPath)
		if err != nil {
			return false, err
		}
		nodeConfig := map[string]interface{}{}
		if partialSync {
			nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
		}
		if network.Kind == models.Fuji {
			globalNetworkFlags.UseFuji = true
		}
		if network.Kind == models.Mainnet {
			globalNetworkFlags.UseMainnet = true
		}
		nodeSettingsLen := max(len(localMachineFlags.StakingSignerKeyPaths), len(localMachineFlags.HTTPPorts), len(localMachineFlags.StakingPorts))
		nodeSettings := make([]localnet.NodeSetting, nodeSettingsLen)
		for i := range nodeSettingsLen {
			nodeSetting := localnet.NodeSetting{}
			if i < len(localMachineFlags.StakingSignerKeyPaths) {
				stakingSignerKey, err := os.ReadFile(localMachineFlags.StakingSignerKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking signer key at %s: %w", localMachineFlags.StakingSignerKeyPaths[i], err)
				}
				stakingCertKey, err := os.ReadFile(localMachineFlags.StakingCertKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking cert key at %s: %w", localMachineFlags.StakingCertKeyPaths[i], err)
				}
				stakingTLSKey, err := os.ReadFile(localMachineFlags.StakingTLSKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking TLS key at %s: %w", localMachineFlags.StakingTLSKeyPaths[i], err)
				}
				nodeSetting.StakingSignerKey = stakingSignerKey
				nodeSetting.StakingCertKey = stakingCertKey
				nodeSetting.StakingTLSKey = stakingTLSKey
			}
			if i < len(localMachineFlags.HTTPPorts) {
				nodeSetting.HTTPPort = uint64(localMachineFlags.HTTPPorts[i])
			}
			if i < len(localMachineFlags.StakingPorts) {
				nodeSetting.StakingPort = uint64(localMachineFlags.StakingPorts[i])
			}
			nodeSettings[i] = nodeSetting
		}
		// anrSettings, avagoVersionSettings, globalNetworkFlags are empty
		if err = node.StartLocalNode(
			app,
			clusterName,
			localMachineFlags.AvagoBinaryPath,
			uint32(bootstrapValidatorFlags.NumBootstrapValidators),
			nodeConfig,
			localnet.ConnectionSettings{},
			nodeSettings,
			avagoVersionSettings,
			network,
		); err != nil {
			return false, err
		}
		clusterNameFlagValue = clusterName
		if len(bootstrapValidatorFlags.BootstrapEndpoints) == 0 {
			bootstrapValidatorFlags.BootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
			if err != nil {
				return false, fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
					"please create your local node again and call blockchain deploy command again", err)
			}
		}
	}
	return false, nil
}

func InitializeValidatorManager(
	blockchainName,
	validatorManagerOwner string,
	subnetID ids.ID,
	blockchainID ids.ID,
	network models.Network,
	avaGoBootstrapValidators []*txs.ConvertSubnetToL1Validator,
	pos bool,
	managerAddress string,
	proxyContractOwner string,
	useACP99 bool,
	useLocalMachine bool,
	signatureAggregatorFlags flags.SignatureAggregatorFlags,
	proofOfStakeFlags flags.POSFlags,
) (bool, error) {
	if useACP99 {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Validator Manager Protocol: ACP99"))
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

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	_, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return tracked, err
	}
	rpcURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		chainSpec,
		true,
		false,
	)
	if err != nil {
		return tracked, err
	}

	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return tracked, err
	}
	if err := client.WaitForEVMBootstrapped(0); err != nil {
		return tracked, err
	}

	ownerAddress := common.HexToAddress(validatorManagerOwner)

	if pos {
		deployed, err := validatormanager.ValidatorProxyHasImplementationSet(rpcURL)
		if err != nil {
			return tracked, err
		}
		if !deployed {
			// it is not in genesis
			ux.Logger.PrintToUser("Deploying Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
			proxyOwnerPrivateKey, err := GetProxyOwnerPrivateKey(
				app,
				network,
				proxyContractOwner,
				ux.Logger.PrintToUser,
			)
			if err != nil {
				return tracked, err
			}
			if useACP99 {
				_, err := validatormanager.DeployAndRegisterValidatorManagerV2_0_0Contract(
					rpcURL,
					genesisPrivateKey,
					proxyOwnerPrivateKey,
				)
				if err != nil {
					return tracked, err
				}
				_, err = validatormanager.DeployAndRegisterPoSValidatorManagerV2_0_0Contract(
					rpcURL,
					genesisPrivateKey,
					proxyOwnerPrivateKey,
				)
				if err != nil {
					return tracked, err
				}
			} else {
				if _, err := validatormanager.DeployAndRegisterPoSValidatorManagerV1_0_0Contract(
					rpcURL,
					genesisPrivateKey,
					proxyOwnerPrivateKey,
				); err != nil {
					return tracked, err
				}
			}
		}
	}

	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return tracked, err
	}

	subnetSDK := blockchainSDK.Subnet{
		SubnetID:            subnetID,
		BlockchainID:        blockchainID,
		OwnerAddress:        &ownerAddress,
		RPC:                 rpcURL,
		BootstrapValidators: avaGoBootstrapValidators,
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
	signatureAggregatorEndpoint, err := signatureaggregator.GetSignatureAggregatorEndpoint()
	if err != nil {
		return tracked, err
	}
	if pos {
		ux.Logger.PrintToUser("Initializing Native Token Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
		found, _, _, managerOwnerPrivateKey, err := contract.SearchForManagedKey(
			app,
			network,
			ownerAddress,
			true,
		)
		if err != nil {
			return tracked, err
		}
		if !found {
			return tracked, fmt.Errorf("could not find validator manager owner private key")
		}
		if err := subnetSDK.InitializeProofOfStake(
			app.Log,
			network.SDKNetwork(),
			genesisPrivateKey,
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
			managerAddress,
			validatormanagerSDK.SpecializationProxyContractAddress,
			managerOwnerPrivateKey,
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
			network.SDKNetwork(),
			genesisPrivateKey,
			aggregatorLogger,
			managerAddress,
			useACP99,
			signatureAggregatorEndpoint,
		); err != nil {
			return tracked, err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	}
	return tracked, nil
}

func convertSubnetToL1(
	bootstrapValidators []models.SubnetValidator,
	deployer *subnet.PublicDeployer,
	subnetID, blockchainID ids.ID,
	network models.Network,
	chain string,
	sidecar models.Sidecar,
	controlKeysList,
	subnetAuthKeysList []string,
	validatorManagerAddressStr string,
	doStrongInputsCheck bool,
) ([]*txs.ConvertSubnetToL1Validator, bool, bool, error) {
	if subnetID == ids.Empty {
		return nil, false, false, constants.ErrNoSubnetID
	}
	if blockchainID == ids.Empty {
		return nil, false, false, constants.ErrNoBlockchainID
	}
	if !common.IsHexAddress(validatorManagerAddressStr) {
		return nil, false, false, constants.ErrInvalidValidatorManagerAddress
	}
	avaGoBootstrapValidators, err := ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return avaGoBootstrapValidators, false, false, err
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)

	if doStrongInputsCheck {
		ux.Logger.PrintToUser("You are about to create a ConvertSubnetToL1Tx on %s with the following content:", network.Name())
		ux.Logger.PrintToUser("  Subnet ID: %s", subnetID)
		ux.Logger.PrintToUser("  Blockchain ID: %s", blockchainID)
		ux.Logger.PrintToUser("  Manager Address: %s", managerAddress.Hex())
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
		blockchainID,
		managerAddress,
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
		validatorManagerAddressStr,
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
		ux.Logger.PrintToUser("Blockchain ID to be used is %s", blockchainID)
		if acceptValue, err := app.Prompt.CaptureYesNo("Is this value correct?"); err != nil {
			return err
		} else if !acceptValue {
			blockchainID = ids.Empty
		}
	}
	if blockchainID == ids.Empty {
		blockchainID, err = app.Prompt.CaptureID("What is the blockchain ID?")
		if err != nil {
			return err
		}
	}

	if validatorManagerAddress == "" {
		validatorManagerAddressAddrFmt, err := app.Prompt.CaptureAddress("What is the address of the Validator Manager?")
		if err != nil {
			return err
		}
		validatorManagerAddress = validatorManagerAddressAddrFmt.String()
	}

	if !convertFlags.ConvertOnly {
		if err = promptValidatorManagementType(app, &sidecar); err != nil {
			return err
		}
		if err := setSidecarValidatorManageOwner(&sidecar, createFlags); err != nil {
			return err
		}
		sidecar.UpdateValidatorManagerAddress(network.Name(), validatorManagerAddress)
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

	err = prepareBootstrapValidators(&bootstrapValidators, network, sidecar, *kc, blockchainName, deployBalance, availableBalance, &convertFlags.LocalMachineFlags, &convertFlags.BootstrapValidatorFlags)
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
		validatorManagerAddress,
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
			sidecar.ValidatorManagerOwner,
			subnetID,
			blockchainID,
			network,
			avaGoBootstrapValidators,
			sidecar.ValidatorManagement == validatormanagertypes.ProofOfStake,
			validatorManagerAddress,
			sidecar.ProxyContractOwner,
			sidecar.UseACP99,
			convertFlags.LocalMachineFlags.UseLocalMachine,
			convertFlags.SigAggFlags,
			convertFlags.ProofOfStakeFlags,
		); err != nil {
			return err
		}
		if sidecar.UseACP99 && sidecar.ValidatorManagement == validatormanagertypes.ProofOfStake {
			sidecar, err := app.LoadSidecar(chain)
			if err != nil {
				return err
			}
			networkInfo := sidecar.Networks[network.Name()]
			networkInfo.ValidatorManagerAddress = validatormanagerSDK.SpecializationProxyContractAddress
			sidecar.Networks[network.Name()] = networkInfo
			if err := app.UpdateSidecar(&sidecar); err != nil {
				return err
			}
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

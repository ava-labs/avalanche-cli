// / Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
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
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/api/info"
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
	SigAggFlags flags.SignatureAggregatorFlags
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
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	flags.AddSignatureAggregatorFlagsToCmd(cmd, &convertFlags.SigAggFlags)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet convert to l1 tx only]")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "auth-keys", nil, "control keys that will be used to authenticate convert to L1 tx")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the convert to L1 tx (for multi-sig)")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")

	cmd.Flags().StringVar(&bootstrapValidatorsJSONFilePath, "bootstrap-filepath", "", "JSON file path that provides details about bootstrap validators, leave Node-ID and BLS values empty if using --generate-node-id=true")
	cmd.Flags().BoolVar(&generateNodeID, "generate-node-id", false, "whether to create new node id for bootstrap validators (Node-ID and BLS values in bootstrap JSON file will be overridden if --bootstrap-filepath flag is used)")
	cmd.Flags().StringSliceVar(&bootstrapEndpoints, "bootstrap-endpoints", nil, "take validator node info from the given endpoints")
	cmd.Flags().BoolVar(&convertOnly, "convert-only", false, "avoid node track, restart and poa manager setup")
	cmd.Flags().BoolVar(&useLocalMachine, "use-local-machine", false, "use local machine as a blockchain validator")
	cmd.Flags().IntVar(&numBootstrapValidators, "num-bootstrap-validators", 0, "(only if --generate-node-id is true) number of bootstrap validators to set up in sovereign L1 validator)")
	cmd.Flags().Float64Var(
		&deployBalanceAVAX,
		"balance",
		float64(constants.BootstrapValidatorBalanceNanoAVAX)/float64(units.Avax),
		"set the AVAX balance of each bootstrap validator that will be used for continuous fee on P-Chain",
	)
	cmd.Flags().UintSliceVar(&httpPorts, "http-port", []uint{}, "http port for node(s)")
	cmd.Flags().UintSliceVar(&stakingPorts, "staking-port", []uint{}, "staking port for node(s)")
	cmd.Flags().StringVar(&changeOwnerAddress, "change-owner-address", "", "address that will receive change if node is no longer L1 validator")

	cmd.Flags().Uint64Var(&poSMinimumStakeAmount, "pos-minimum-stake-amount", 1, "minimum stake amount")
	cmd.Flags().Uint64Var(&poSMaximumStakeAmount, "pos-maximum-stake-amount", 1000, "maximum stake amount")
	cmd.Flags().Uint64Var(&poSMinimumStakeDuration, "pos-minimum-stake-duration", constants.PoSL1MinimumStakeDurationSeconds, "minimum stake duration (in seconds)")
	cmd.Flags().Uint16Var(&poSMinimumDelegationFee, "pos-minimum-delegation-fee", 1, "minimum delegation fee")
	cmd.Flags().Uint8Var(&poSMaximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "maximum stake multiplier")
	cmd.Flags().Uint64Var(&poSWeightToValueFactor, "pos-weight-to-value-factor", 1, "weight to value factor")

	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "set primary network partial sync for new validators")

	cmd.Flags().BoolVar(&createFlags.proofOfAuthority, "proof-of-authority", false, "use proof of authority(PoA) for validator management")
	cmd.Flags().BoolVar(&createFlags.proofOfStake, "proof-of-stake", false, "use proof of stake(PoS) for validator management")
	cmd.Flags().StringVar(&createFlags.validatorManagerOwner, "validator-manager-owner", "", "EVM address that controls Validator Manager Owner")
	cmd.Flags().StringVar(&createFlags.proxyContractOwner, "proxy-contract-owner", "", "EVM address that controls ProxyAdmin for TransparentProxy of ValidatorManager contract")
	cmd.Flags().Uint64Var(&createFlags.rewardBasisPoints, "reward-basis-points", 100, "(PoS only) reward basis points for PoS Reward Calculator")
	cmd.Flags().StringVar(&validatorManagerAddress, "validator-manager-address", "", "validator manager address")
	cmd.Flags().BoolVar(&doStrongInputChecks, "verify-input", true, "check for input confirmation")
	return cmd
}

func StartLocalMachine(
	network models.Network,
	sidecar models.Sidecar,
	blockchainName string,
	deployBalance,
	availableBalance uint64,
	httpPorts []uint,
	stakingPorts []uint,
	numBootstrapValidator int,
) (bool, error) {
	var err error
	if network.Kind == models.Local {
		useLocalMachine = true
	}
	networkNameComponent := strings.ReplaceAll(strings.ToLower(network.Name()), " ", "-")
	clusterName := fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent)
	if clusterNameFlagValue != "" {
		clusterName = clusterNameFlagValue
		if localnet.LocalClusterExists(app, clusterName) {
			useLocalMachine = true
			if len(bootstrapEndpoints) == 0 {
				bootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
				if err != nil {
					return false, fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
						"please create your local node again and call blockchain deploy command again", err)
				}
			}
			network = models.ConvertClusterToNetwork(network)
		}
	}
	// ask user if we want to use local machine if cluster is not provided
	if !useLocalMachine && clusterNameFlagValue == "" {
		ux.Logger.PrintToUser("You can use your local machine as a bootstrap validator on the blockchain")
		ux.Logger.PrintToUser("This means that you don't have to to set up a remote server on a cloud service (e.g. AWS / GCP) to be a validator on the blockchain.")

		useLocalMachine, err = app.Prompt.CaptureYesNo("Do you want to use your local machine as a bootstrap validator?")
		if err != nil {
			return false, err
		}
	}
	// default number of local machine nodes to be 1
	// we set it here instead of at flag level so that we don't prompt if user wants to use local machine when they set numLocalNodes flag value
	if useLocalMachine && numBootstrapValidator == 0 {
		numBootstrapValidator = constants.DefaultNumberOfLocalMachineNodes
	}
	// if no cluster provided - we create one with fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent) name
	if useLocalMachine && clusterNameFlagValue == "" {
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
		requiredBalance := deployBalance * uint64(numBootstrapValidator)
		if availableBalance < requiredBalance {
			return false, fmt.Errorf(
				"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
				numBootstrapValidator,
				requiredBalance,
				availableBalance,
			)
		}
		avagoVersionSettings := node.AvalancheGoVersionSettings{}
		// setup (install if needed) avalanchego binary
		avagoVersion := userProvidedAvagoVersion
		if userProvidedAvagoVersion == constants.DefaultAvalancheGoVersion && avagoBinaryPath == "" {
			// nothing given: get avago version from RPC compat
			avagoVersion, err = vm.GetLatestAvalancheGoByProtocolVersion(
				app,
				sidecar.RPCVersion,
				constants.AvalancheGoCompatibilityURL,
			)
			if err != nil {
				if err != vm.ErrNoAvagoVersion {
					return false, err
				}
				avagoVersion = constants.LatestPreReleaseVersionTag
			}
		}
		avagoBinaryPath, err := localnet.SetupAvalancheGoBinary(app, avagoVersion, avagoBinaryPath)
		if err != nil {
			return false, err
		}
		nodeConfig := map[string]interface{}{}
		if app.AvagoNodeConfigExists(blockchainName) {
			nodeConfig, err = utils.ReadJSON(app.GetAvagoNodeConfigPath(blockchainName))
			if err != nil {
				return false, err
			}
		}
		if partialSync {
			nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
		}
		if network.Kind == models.Fuji {
			globalNetworkFlags.UseFuji = true
		}
		if network.Kind == models.Mainnet {
			globalNetworkFlags.UseMainnet = true
		}
		nodeSettingsLen := max(len(httpPorts), len(stakingPorts))
		nodeSettings := make([]localnet.NodeSetting, nodeSettingsLen)
		for i := range nodeSettingsLen {
			nodeSetting := localnet.NodeSetting{}
			if i < len(httpPorts) {
				nodeSetting.HTTPPort = uint64(httpPorts[i])
			}
			if i < len(stakingPorts) {
				nodeSetting.StakingPort = uint64(stakingPorts[i])
			}
			nodeSettings[i] = nodeSetting
		}
		// anrSettings, avagoVersionSettings, globalNetworkFlags are empty
		if err = node.StartLocalNode(
			app,
			clusterName,
			avagoBinaryPath,
			uint32(numBootstrapValidator),
			nodeConfig,
			localnet.ConnectionSettings{},
			nodeSettings,
			avagoVersionSettings,
			network,
		); err != nil {
			return false, err
		}
		clusterNameFlagValue = clusterName
		if len(bootstrapEndpoints) == 0 {
			bootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
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
	validatorManagerAddrStr string,
	proxyContractOwner string,
	useACP99 bool,
	signatureAggregatorFlags flags.SignatureAggregatorFlags,
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

	if pos {
		deployed, err := validatormanager.ProxyHasValidatorManagerSet(rpcURL)
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
			if _, err := validatormanager.DeployAndRegisterPoSValidatorManagerContrac(
				rpcURL,
				genesisPrivateKey,
				proxyOwnerPrivateKey,
			); err != nil {
				return tracked, err
			}
		}
	}

	extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName)
	if err != nil {
		return tracked, err
	}

	ownerAddress := common.HexToAddress(validatorManagerOwner)
	subnetSDK := blockchainSDK.Subnet{
		SubnetID:            subnetID,
		BlockchainID:        blockchainID,
		OwnerAddress:        &ownerAddress,
		RPC:                 rpcURL,
		BootstrapValidators: avaGoBootstrapValidators,
	}
	aggregatorLogger, err := signatureaggregator.NewSignatureAggregatorLoggerNewLogger(
		signatureAggregatorFlags.AggregatorLogLevel,
		signatureAggregatorFlags.AggregatorLogToStdout,
		app.GetAggregatorLogDir(clusterName),
	)
	if err != nil {
		return tracked, err
	}
	aggregatorCtx, aggregatorCancel := sdkutils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	if pos {
		ux.Logger.PrintToUser("Initializing Native Token Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
		if err := subnetSDK.InitializeProofOfStake(
			aggregatorCtx,
			app.Log,
			network.SDKNetwork(),
			genesisPrivateKey,
			extraAggregatorPeers,
			aggregatorLogger,
			validatorManagerSDK.PoSParams{
				MinimumStakeAmount:      big.NewInt(int64(poSMinimumStakeAmount)),
				MaximumStakeAmount:      big.NewInt(int64(poSMaximumStakeAmount)),
				MinimumStakeDuration:    poSMinimumStakeDuration,
				MinimumDelegationFee:    poSMinimumDelegationFee,
				MaximumStakeMultiplier:  poSMaximumStakeMultiplier,
				WeightToValueFactor:     big.NewInt(int64(poSWeightToValueFactor)),
				RewardCalculatorAddress: validatorManagerSDK.RewardCalculatorAddress,
				UptimeBlockchainID:      blockchainID,
			},
			validatorManagerAddrStr,
		); err != nil {
			return tracked, err
		}
		ux.Logger.GreenCheckmarkToUser("Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
	} else {
		ux.Logger.PrintToUser("Initializing Proof of Authority Validator Manager contract on blockchain %s ...", blockchainName)
		if err := subnetSDK.InitializeProofOfAuthority(
			aggregatorCtx,
			app.Log,
			network.SDKNetwork(),
			genesisPrivateKey,
			extraAggregatorPeers,
			aggregatorLogger,
			validatorManagerAddrStr,
			useACP99,
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
func convertBlockchain(_ *cobra.Command, args []string) error {
	blockchainName := args[0]

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	var bootstrapValidators []models.SubnetValidator
	if bootstrapValidatorsJSONFilePath != "" {
		bootstrapValidators, err = LoadBootstrapValidator(bootstrapValidatorsJSONFilePath)
		if err != nil {
			return err
		}
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

	if !convertOnly {
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

	deployBalance := uint64(deployBalanceAVAX * float64(units.Avax))

	if len(bootstrapValidators) == 0 {
		if changeOwnerAddress == "" {
			// use provided key as change owner unless already set
			if pAddr, err := kc.PChainFormattedStrAddresses(); err == nil && len(pAddr) > 0 {
				changeOwnerAddress = pAddr[0]
				ux.Logger.PrintToUser("Using [%s] to be set as a change owner for leftover AVAX", changeOwnerAddress)
			}
		}
		if !generateNodeID {
			if cancel, err := StartLocalMachine(
				network,
				sidecar,
				blockchainName,
				deployBalance,
				availableBalance,
				httpPorts,
				stakingPorts,
				numBootstrapValidators,
			); err != nil {
				return err
			} else if cancel {
				return nil
			}
		}
		switch {
		case len(bootstrapEndpoints) > 0:
			if changeOwnerAddress == "" {
				changeOwnerAddress, err = blockchain.GetKeyForChangeOwner(app, network)
				if err != nil {
					return err
				}
			}
			for _, endpoint := range bootstrapEndpoints {
				infoClient := info.NewClient(endpoint)
				ctx, cancel := utils.GetAPILargeContext()
				defer cancel()
				nodeID, proofOfPossession, err := infoClient.GetNodeID(ctx)
				if err != nil {
					return err
				}
				publicKey = "0x" + hex.EncodeToString(proofOfPossession.PublicKey[:])
				pop = "0x" + hex.EncodeToString(proofOfPossession.ProofOfPossession[:])

				bootstrapValidators = append(bootstrapValidators, models.SubnetValidator{
					NodeID:               nodeID.String(),
					Weight:               constants.BootstrapValidatorWeight,
					Balance:              deployBalance,
					BLSPublicKey:         publicKey,
					BLSProofOfPossession: pop,
					ChangeOwnerAddr:      changeOwnerAddress,
				})
			}
		case clusterNameFlagValue != "":
			// for remote clusters we don't need to ask for bootstrap validators and can read it from filesystem
			bootstrapValidators, err = getClusterBootstrapValidators(clusterNameFlagValue, network, deployBalance)
			if err != nil {
				return fmt.Errorf("error getting bootstrap validators from cluster %s: %w", clusterNameFlagValue, err)
			}

		default:
			bootstrapValidators, err = promptBootstrapValidators(
				network,
				changeOwnerAddress,
				numBootstrapValidators,
				deployBalance,
				availableBalance,
			)
			if err != nil {
				return err
			}
		}
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

	if !convertOnly && !generateNodeID {
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
			convertFlags.SigAggFlags,
		); err != nil {
			return err
		}
	} else {
		ux.Logger.GreenCheckmarkToUser("Converted blockchain successfully generated")
		ux.Logger.PrintToUser("To finish conversion to sovereign L1, create the corresponding Avalanche node(s) with the provided Node ID and BLS Info")
		ux.Logger.PrintToUser("Created Node ID and BLS Info can be found at %s", app.GetSidecarPath(blockchainName))
		ux.Logger.PrintToUser("Once the Avalanche Node(s) are created and are tracking the blockchain, call `avalanche contract initValidatorManager %s` to finish conversion to sovereign L1", blockchainName)
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Your L1 is ready for on-chain interactions."))
	ux.Logger.PrintToUser("")
	ux.Logger.GreenCheckmarkToUser("Subnet is successfully converted to sovereign L1")

	return nil
}

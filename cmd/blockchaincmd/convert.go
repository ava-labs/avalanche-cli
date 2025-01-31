// / Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/blockchain"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// avalanche blockchain deploy
func newConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "convert [blockchainName]",
		Short:             "Converts a Subnet into a sovereign L1",
		Long:              `The blockchain converts command converts a Subnet into sovereign L1.`,
		RunE:              convertBlockchain,
		PersistentPostRun: handlePostRun,
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	privateKeyFlags.SetFlagNames("blockchain-private-key", "blockchain-key", "blockchain-genesis-key")
	privateKeyFlags.AddToCmd(cmd, "to fund validator manager initialization")
	cmd.Flags().StringVar(
		&userProvidedAvagoVersion,
		"avalanchego-version",
		constants.DefaultAvalancheGoVersion,
		"use this version of avalanchego (ex: v1.17.12)",
	)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use the fee-paying key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to make blockchain changes")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may make blockchain changes")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "auth-keys", nil, "control keys that will be used to authenticate chain creation")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the blockchain creation tx")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVarP(&subnetIDStr, "subnet-id", "u", "", "do not create a subnet, deploy the blockchain into the given subnet id")
	cmd.Flags().Uint32Var(&mainnetChainID, "mainnet-chain-id", 0, "use different ChainID for mainnet deployment")
	cmd.Flags().StringVar(&avagoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().BoolVar(&subnetOnly, "subnet-only", false, "only create a subnet")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-local-teleporter", false, "skip automatic ICM deploy on local networks [to be deprecated]")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-teleporter-deploy", false, "skip automatic ICM deploy")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-icm-deploy", false, "skip automatic ICM deploy")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "noicm", false, "skip automatic ICM deploy")
	cmd.Flags().BoolVar(&icmSpec.SkipRelayerDeploy, skipRelayerFlagName, false, "skip relayer deploy")
	cmd.Flags().StringVar(
		&icmSpec.ICMVersion,
		"teleporter-version",
		constants.LatestReleaseVersionTag,
		"ICM version to deploy",
	)
	cmd.Flags().StringVar(
		&icmSpec.ICMVersion,
		"icm-version",
		constants.LatestReleaseVersionTag,
		"ICM version to deploy",
	)
	cmd.Flags().StringVar(
		&icmSpec.RelayerVersion,
		"relayer-version",
		constants.LatestPreReleaseVersionTag,
		"relayer version to deploy",
	)
	cmd.Flags().StringVar(&icmSpec.RelayerBinPath, "relayer-path", "", "relayer binary to use")
	cmd.Flags().StringVar(&icmSpec.RelayerLogLevel, "relayer-log-level", "info", "log level to be used for relayer logs")
	cmd.Flags().Float64Var(&relayerAmount, "relayer-amount", 0, "automatically fund relayer fee payments with the given amount")
	cmd.Flags().StringVar(&relayerKeyName, "relayer-key", "", "key to be used by default both for rewards and to pay fees")
	cmd.Flags().StringVar(&icmKeyName, "icm-key", constants.ICMKeyName, "key to be used to pay for ICM deploys")
	cmd.Flags().StringVar(&cchainIcmKeyName, "cchain-icm-key", "", "key to be used to pay for ICM deploys on C-Chain")
	cmd.Flags().BoolVar(&relayCChain, "relay-cchain", true, "relay C-Chain as source and destination")
	cmd.Flags().StringVar(&cChainFundingKey, "cchain-funding-key", "", "key to be used to fund relayer account on cchain")
	cmd.Flags().BoolVar(&relayerAllowPrivateIPs, "relayer-allow-private-ips", true, "allow relayer to connec to private ips")
	cmd.Flags().StringVar(&icmSpec.MessengerContractAddressPath, "teleporter-messenger-contract-address-path", "", "path to an ICM Messenger contract address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerAddressPath, "teleporter-messenger-deployer-address-path", "", "path to an ICM Messenger deployer address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerTxPath, "teleporter-messenger-deployer-tx-path", "", "path to an ICM Messenger deployer tx file")
	cmd.Flags().StringVar(&icmSpec.RegistryBydecodePath, "teleporter-registry-bytecode-path", "", "path to an ICM Registry bytecode file")
	cmd.Flags().StringVar(&bootstrapValidatorsJSONFilePath, "bootstrap-filepath", "", "JSON file path that provides details about bootstrap validators, leave Node-ID and BLS values empty if using --generate-node-id=true")
	cmd.Flags().BoolVar(&generateNodeID, "generate-node-id", false, "whether to create new node id for bootstrap validators (Node-ID and BLS values in bootstrap JSON file will be overridden if --bootstrap-filepath flag is used)")
	cmd.Flags().StringSliceVar(&bootstrapEndpoints, "bootstrap-endpoints", nil, "take validator node info from the given endpoints")
	cmd.Flags().BoolVar(&convertOnly, "convert-only", false, "avoid node track, restart and poa manager setup")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&aggregatorLogToStdout, "aggregator-log-to-stdout", false, "use stdout for signature aggregator logs")
	cmd.Flags().StringSliceVar(&aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().BoolVar(&aggregatorAllowPrivatePeers, "aggregator-allow-private-peers", true, "allow the signature aggregator to connect to peers with private IP")
	cmd.Flags().BoolVar(&useLocalMachine, "use-local-machine", false, "use local machine as a blockchain validator")
	cmd.Flags().IntVar(&numBootstrapValidators, "num-bootstrap-validators", 0, "(only if --generate-node-id is true) number of bootstrap validators to set up in sovereign L1 validator)")
	cmd.Flags().Float64Var(
		&deployBalanceAVAX,
		"balance",
		float64(constants.BootstrapValidatorBalanceNanoAVAX)/float64(units.Avax),
		"set the AVAX balance of each bootstrap validator that will be used for continuous fee on P-Chain",
	)
	cmd.Flags().IntVar(&numLocalNodes, "num-local-nodes", 0, "number of nodes to be created on local machine")
	cmd.Flags().StringVar(&changeOwnerAddress, "change-owner-address", "", "address that will receive change if node is no longer L1 validator")

	cmd.Flags().Uint64Var(&poSMinimumStakeAmount, "pos-minimum-stake-amount", 1, "minimum stake amount")
	cmd.Flags().Uint64Var(&poSMaximumStakeAmount, "pos-maximum-stake-amount", 1000, "maximum stake amount")
	cmd.Flags().Uint64Var(&poSMinimumStakeDuration, "pos-minimum-stake-duration", constants.PoSL1MinimumStakeDurationSeconds, "minimum stake duration (in seconds)")
	cmd.Flags().Uint16Var(&poSMinimumDelegationFee, "pos-minimum-delegation-fee", 1, "minimum delegation fee")
	cmd.Flags().Uint8Var(&poSMaximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "maximum stake multiplier")
	cmd.Flags().Uint64Var(&poSWeightToValueFactor, "pos-weight-to-value-factor", 1, "weight to value factor")

	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "set primary network partial sync for new validators")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", constants.LocalNetworkNumNodes, "number of nodes to be created on local network deploy")
	return cmd
}

// convertBlockchain is the cobra command run for converting subnets into sovereign L1
func convertBlockchain(cmd *cobra.Command, args []string) error {
	blockchainName := args[0]

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	if icmSpec.MessengerContractAddressPath != "" || icmSpec.MessengerDeployerAddressPath != "" || icmSpec.MessengerDeployerTxPath != "" || icmSpec.RegistryBydecodePath != "" {
		if icmSpec.MessengerContractAddressPath == "" || icmSpec.MessengerDeployerAddressPath == "" || icmSpec.MessengerDeployerTxPath == "" || icmSpec.RegistryBydecodePath == "" {
			return fmt.Errorf("if setting any ICM asset path, you must set all ICM asset paths")
		}
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

	if sidecar.ImportedFromAPM {
		return errors.New("unable to deploy blockchains imported from a repo")
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
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	clusterNameFlagValue = globalNetworkFlags.ClusterName

	subnetID := sidecar.Networks[network.Name()].SubnetID
	blockchainID := sidecar.Networks[network.Name()].BlockchainID

	isEVMGenesis, validationErr, err := app.HasSubnetEVMGenesis(chain)
	if err != nil {
		return err
	}
	if sidecar.VM == models.SubnetEvm && !isEVMGenesis {
		return fmt.Errorf("failed to validate SubnetEVM genesis format: %w", validationErr)
	}

	chainGenesis, err := app.LoadRawGenesis(chain)
	if err != nil {
		return err
	}

	if isEVMGenesis {
		// is is a subnet evm or a custom vm based on subnet evm
		if network.Kind == models.Mainnet {
			err = getSubnetEVMMainnetChainID(&sidecar, chain)
			if err != nil {
				return err
			}
			chainGenesis, err = updateSubnetEVMGenesisChainID(chainGenesis, sidecar.SubnetEVMMainnetChainID)
			if err != nil {
				return err
			}
		}
		err = checkSubnetEVMDefaultAddressNotInAlloc(network, chain)
		if err != nil {
			return err
		}
	}

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

	if changeOwnerAddress == "" {
		// use provided key as change owner unless already set
		if pAddr, err := kc.PChainFormattedStrAddresses(); err == nil && len(pAddr) > 0 {
			changeOwnerAddress = pAddr[0]
			ux.Logger.PrintToUser("Using [%s] to be set as a change owner for leftover AVAX", changeOwnerAddress)
		}
	}
	if !generateNodeID {
		if network.Kind == models.Local {
			useLocalMachine = true
		}
		networkNameComponent := strings.ReplaceAll(strings.ToLower(network.Name()), " ", "-")
		clusterName := fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent)
		if clusterNameFlagValue != "" {
			clusterName = clusterNameFlagValue
			clusterConfig, err := app.GetClusterConfig(clusterName)
			if err != nil {
				return err
			}
			// check if cluster is local
			if clusterConfig.Local {
				useLocalMachine = true
				if len(bootstrapEndpoints) == 0 {
					bootstrapEndpoints, err = getLocalBootstrapEndpoints()
					if err != nil {
						return fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
							"please create your local node again and call blockchain deploy command again", err)
					}
				}
				network = models.ConvertClusterToNetwork(network)
			}
		}
		if numLocalNodes > 0 {
			useLocalMachine = true
		}
		// ask user if we want to use local machine if cluster is not provided
		if !useLocalMachine && clusterNameFlagValue == "" {
			ux.Logger.PrintToUser("You can use your local machine as a bootstrap validator on the blockchain")
			ux.Logger.PrintToUser("This means that you don't have to to set up a remote server on a cloud service (e.g. AWS / GCP) to be a validator on the blockchain.")

			useLocalMachine, err = app.Prompt.CaptureYesNo("Do you want to use your local machine as a bootstrap validator?")
			if err != nil {
				return err
			}
		}
		// default number of local machine nodes to be 1
		// we set it here instead of at flag level so that we don't prompt if user wants to use local machine when they set numLocalNodes flag value
		if useLocalMachine && numLocalNodes == 0 {
			numLocalNodes = constants.DefaultNumberOfLocalMachineNodes
		}
		// if no cluster provided - we create one with fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent) name
		if useLocalMachine && clusterNameFlagValue == "" {
			if clusterExists, err := node.CheckClusterIsLocal(app, clusterName); err != nil {
				return err
			} else if clusterExists {
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
					return err
				}
				if !yes {
					return nil
				}
				_ = node.DestroyLocalNode(app, clusterName)
			}
			requiredBalance := deployBalance * uint64(numLocalNodes)
			if availableBalance < requiredBalance {
				return fmt.Errorf(
					"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
					numLocalNodes,
					requiredBalance,
					availableBalance,
				)
			}
			// stop local avalanchego process so that we can generate new local cluster
			_ = node.StopLocalNode(app)
			anrSettings := node.ANRSettings{}
			avagoVersionSettings := node.AvalancheGoVersionSettings{}
			if avagoBinaryPath == "" {
				useLatestAvalanchegoPreReleaseVersion := true
				useLatestAvalanchegoReleaseVersion := false
				if userProvidedAvagoVersion != constants.DefaultAvalancheGoVersion {
					useLatestAvalanchegoReleaseVersion = false
					useLatestAvalanchegoPreReleaseVersion = false
				} else {
					userProvidedAvagoVersion = ""
				}
				avaGoVersionSetting := node.AvalancheGoVersionSettings{
					UseCustomAvalanchegoVersion:           userProvidedAvagoVersion,
					UseLatestAvalanchegoPreReleaseVersion: useLatestAvalanchegoPreReleaseVersion,
					UseLatestAvalanchegoReleaseVersion:    useLatestAvalanchegoReleaseVersion,
				}
				avalancheGoVersion, err := node.GetAvalancheGoVersion(app, avaGoVersionSetting)
				if err != nil {
					return err
				}
				_, avagoDir, err := binutils.SetupAvalanchego(app, avalancheGoVersion)
				if err != nil {
					return fmt.Errorf("failed installing Avalanche Go version %s: %w", avalancheGoVersion, err)
				}
				avagoBinaryPath = filepath.Join(avagoDir, "avalanchego")
			}
			nodeConfig := map[string]interface{}{}
			if app.AvagoNodeConfigExists(blockchainName) {
				nodeConfig, err = utils.ReadJSON(app.GetAvagoNodeConfigPath(blockchainName))
				if err != nil {
					return err
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
			// anrSettings, avagoVersionSettings, globalNetworkFlags are empty
			if err = node.StartLocalNode(
				app,
				clusterName,
				avagoBinaryPath,
				uint32(numLocalNodes),
				nodeConfig,
				anrSettings,
				avagoVersionSettings,
				network,
				networkoptions.NetworkFlags{},
				nil,
			); err != nil {
				return err
			}
			clusterNameFlagValue = clusterName
			if len(bootstrapEndpoints) == 0 {
				bootstrapEndpoints, err = getLocalBootstrapEndpoints()
				if err != nil {
					return fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
						"please create your local node again and call blockchain deploy command again", err)
				}
			}
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

	// from here on we are assuming a public deploy
	if subnetOnly && subnetIDStr != "" {
		return errMutuallyExlusiveSubnetFlags
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

	network.HandlePublicNetworkSimulation()

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for blockchain tx signing
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
	ux.Logger.PrintToUser("Your blockchain auth keys for chain creation: %s", subnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, kc, network)

	var (
		savePartialTx           bool
		tx                      *txs.Tx
		remainingSubnetAuthKeys []string
		isFullySigned           bool
	)
	avaGoBootstrapValidators, err := ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
	if err != nil {
		return err
	}
	deployer.CleanCacheWallet()
	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
	isFullySigned, convertL1TxID, tx, remainingSubnetAuthKeys, err := deployer.ConvertL1(
		controlKeys,
		subnetAuthKeys,
		subnetID,
		blockchainID,
		managerAddress,
		avaGoBootstrapValidators,
	)
	if err != nil {
		ux.Logger.RedXToUser("error converting blockchain: %s. fix the issue and try again with a new convert cmd", err)
		return err
	}

	savePartialTx = !isFullySigned && err == nil
	ux.Logger.PrintToUser("ConvertSubnetToL1Tx ID: %s", convertL1TxID)

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
			return err
		}
	}

	_, err = ux.TimedProgressBar(
		30*time.Second,
		"Waiting for the Subnet to be converted into a sovereign L1 ...",
		0,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	setBootstrapValidatorValidationID(avaGoBootstrapValidators, bootstrapValidators, subnetID)
	if err := app.UpdateSidecarNetworks(
		&sidecar,
		network,
		subnetID,
		blockchainID,
		"",
		"",
		bootstrapValidators,
		clusterNameFlagValue,
	); err != nil {
		return err
	}

	if !convertOnly && !generateNodeID {
		clusterName := clusterNameFlagValue
		switch {
		case useLocalMachine:
			if err := node.TrackSubnetWithLocalMachine(
				app,
				clusterName,
				blockchainName,
				avagoBinaryPath,
			); err != nil {
				return err
			}
		default:
			if clusterName != "" {
				if err = node.SyncSubnet(app, clusterName, blockchainName, true, nil); err != nil {
					return err
				}

				if err := node.WaitForHealthyCluster(app, clusterName, node.HealthCheckTimeout, node.HealthCheckPoolTime); err != nil {
					return err
				}
			}
		}
		chainSpec := contract.ChainSpec{
			BlockchainName: blockchainName,
		}
		_, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
			app,
			network,
			chainSpec,
		)
		if err != nil {
			return err
		}
		rpcURL, _, err := contract.GetBlockchainEndpoints(
			app,
			network,
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return err
		}
		client, err := evm.GetClient(rpcURL)
		if err != nil {
			return err
		}
		evm.WaitForChainID(client)
		extraAggregatorPeers, err := blockchain.GetAggregatorExtraPeers(app, clusterName, aggregatorExtraEndpoints)
		if err != nil {
			return err
		}

		ownerAddress := common.HexToAddress(sidecar.ValidatorManagerOwner)
		subnetSDK := blockchainSDK.Subnet{
			SubnetID:            subnetID,
			BlockchainID:        blockchainID,
			OwnerAddress:        &ownerAddress,
			RPC:                 rpcURL,
			BootstrapValidators: avaGoBootstrapValidators,
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
		if sidecar.ValidatorManagement == models.ProofOfStake {
			ux.Logger.PrintToUser("Initializing Native Token Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
			if err := subnetSDK.InitializeProofOfStake(
				network,
				genesisPrivateKey,
				extraAggregatorPeers,
				aggregatorAllowPrivatePeers,
				aggregatorLogger,
				validatorManagerSDK.PoSParams{
					MinimumStakeAmount:      big.NewInt(int64(poSMinimumStakeAmount)),
					MaximumStakeAmount:      big.NewInt(int64(poSMaximumStakeAmount)),
					MinimumStakeDuration:    poSMinimumStakeDuration,
					MinimumDelegationFee:    poSMinimumDelegationFee,
					MaximumStakeMultiplier:  poSMaximumStakeMultiplier,
					WeightToValueFactor:     big.NewInt(int64(poSWeightToValueFactor)),
					RewardCalculatorAddress: validatorManagerSDK.RewardCalculatorAddress,
				},
			); err != nil {
				return err
			}
			ux.Logger.GreenCheckmarkToUser("Proof of Stake Validator Manager contract successfully initialized on blockchain %s", blockchainName)
		} else {
			ux.Logger.PrintToUser("Initializing Proof of Authority Validator Manager contract on blockchain %s ...", blockchainName)
			if err := subnetSDK.InitializeProofOfAuthority(
				network,
				genesisPrivateKey,
				extraAggregatorPeers,
				aggregatorAllowPrivatePeers,
				aggregatorLogger,
			); err != nil {
				return err
			}
			ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
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

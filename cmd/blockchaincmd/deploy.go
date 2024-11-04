// / Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/network/peer"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	avagoutils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

var deploySupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.EtnaDevnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

var (
	sameControlKey                  bool
	keyName                         string
	threshold                       uint32
	controlKeys                     []string
	subnetAuthKeys                  []string
	userProvidedAvagoVersion        string
	outputTxPath                    string
	useLedger                       bool
	useLocalMachine                 bool
	useEwoq                         bool
	ledgerAddresses                 []string
	sovereign                       bool
	subnetIDStr                     string
	mainnetChainID                  uint32
	skipCreatePrompt                bool
	avagoBinaryPath                 string
	numBootstrapValidators          int
	numLocalNodes                   int
	changeOwnerAddress              string
	subnetOnly                      bool
	icmSpec                         subnet.ICMSpec
	generateNodeID                  bool
	bootstrapValidatorsJSONFilePath string
	privateKeyFlags                 contract.PrivateKeyFlags
	bootstrapEndpoints              []string
	convertOnly                     bool

	errMutuallyExlusiveControlKeys = errors.New("--control-keys and --same-control-key are mutually exclusive")
	ErrMutuallyExlusiveKeyLedger   = errors.New("key source flags --key, --ledger/--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet          = errors.New("key --key is not available for mainnet operations")
	errMutuallyExlusiveSubnetFlags = errors.New("--subnet-only and --subnet-id are mutually exclusive")
)

// avalanche blockchain deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [blockchainName]",
		Short: "Deploys a blockchain configuration",
		Long: `The blockchain deploy command deploys your Blockchain configuration locally, to Fuji Testnet, or to Mainnet.

At the end of the call, the command prints the RPC URL you can use to interact with the Subnet.

Avalanche-CLI only supports deploying an individual Blockchain once per network. Subsequent
attempts to deploy the same Blockchain to the same network (local, Fuji, Mainnet) aren't
allowed. If you'd like to redeploy a Blockchain locally for testing, you must first call
avalanche network clean to reset all deployed chain state. Subsequent local deploys
redeploy the chain with fresh state. You can deploy the same Blockchain to multiple networks,
so you can take your locally tested Subnet and deploy it on Fuji or Mainnet.`,
		RunE:              deployBlockchain,
		PersistentPostRun: handlePostRun,
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, deploySupportedNetworkOptions)
	privateKeyFlags.SetFlagNames("blockchain-private-key", "blockchain-key", "blockchain-genesis-key")
	privateKeyFlags.AddToCmd(cmd, "to fund validator manager initialization")
	cmd.Flags().StringVar(&userProvidedAvagoVersion, "avalanchego-version", "latest", "use this version of avalanchego (ex: v1.17.12)")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&sameControlKey, "same-control-key", "s", false, "use the fee-paying key as control key")
	cmd.Flags().Uint32Var(&threshold, "threshold", 0, "required number of control key signatures to make subnet changes")
	cmd.Flags().StringSliceVar(&controlKeys, "control-keys", nil, "addresses that may make subnet changes")
	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "control keys that will be used to authenticate chain creation")
	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "file path of the blockchain creation tx")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet deploy only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().StringVarP(&subnetIDStr, "subnet-id", "u", "", "do not create a subnet, deploy the blockchain into the given subnet id")
	cmd.Flags().Uint32Var(&mainnetChainID, "mainnet-chain-id", 0, "use different ChainID for mainnet deployment")
	cmd.Flags().StringVar(&avagoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().BoolVar(&subnetOnly, "subnet-only", false, "only create a subnet")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-local-teleporter", false, "skip automatic teleporter deploy on local networks [to be deprecated]")
	cmd.Flags().BoolVar(&icmSpec.SkipICMDeploy, "skip-teleporter-deploy", false, "skip automatic teleporter deploy")
	cmd.Flags().BoolVar(&icmSpec.SkipRelayerDeploy, "skip-relayer", false, "skip relayer deploy")
	cmd.Flags().StringVar(&icmSpec.ICMVersion, "teleporter-version", "latest", "teleporter version to deploy")
	cmd.Flags().StringVar(&icmSpec.RelayerVersion, "relayer-version", "latest", "relayer version to deploy")
	cmd.Flags().StringVar(&icmSpec.MessengerContractAddressPath, "teleporter-messenger-contract-address-path", "", "path to an interchain messenger contract address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerAddressPath, "teleporter-messenger-deployer-address-path", "", "path to an interchain messenger deployer address file")
	cmd.Flags().StringVar(&icmSpec.MessengerDeployerTxPath, "teleporter-messenger-deployer-tx-path", "", "path to an interchain messenger deployer tx file")
	cmd.Flags().StringVar(&icmSpec.RegistryBydecodePath, "teleporter-registry-bytecode-path", "", "path to an interchain messenger registry bytecode file")
	cmd.Flags().StringVar(&bootstrapValidatorsJSONFilePath, "bootstrap-filepath", "", "JSON file path that provides details about bootstrap validators, leave Node-ID and BLS values empty if using --generate-node-id=true")
	cmd.Flags().BoolVar(&generateNodeID, "generate-node-id", false, "whether to create new node id for bootstrap validators (Node-ID and BLS values in bootstrap JSON file will be overridden if --bootstrap-filepath flag is used)")
	cmd.Flags().StringSliceVar(&bootstrapEndpoints, "bootstrap-endpoints", nil, "take validator node info from the given endpoints")
	cmd.Flags().BoolVar(&convertOnly, "convert-only", false, "avoid node track, restart and poa manager setup")
	cmd.Flags().StringVar(&aggregatorLogLevel, "aggregator-log-level", "Off", "log level to use with signature aggregator")
	cmd.Flags().StringSliceVar(&aggregatorExtraEndpoints, "aggregator-extra-endpoints", nil, "endpoints for extra nodes that are needed in signature aggregation")
	cmd.Flags().BoolVar(&useLocalMachine, "use-local-machine", false, "use local machine as a blockchain validator")
	cmd.Flags().IntVar(&numBootstrapValidators, "num-bootstrap-validators", 0, "(only if --generate-node-id is true) number of bootstrap validators to set up in sovereign L1 validator)")
	cmd.Flags().IntVar(&numLocalNodes, "num-local-nodes", 5, "number of nodes to be created on local machine")
	cmd.Flags().StringVar(&changeOwnerAddress, "change-owner-address", "", "address that will receive change if node is no longer L1 validator")
	return cmd
}

type SubnetValidator struct {
	// Must be Ed25519 NodeID
	NodeID ids.NodeID `json:"nodeID"`
	// Weight of this validator used when sampling
	Weight uint64 `json:"weight"`
	// When this validator will stop validating the Subnet
	EndTime uint64 `json:"endTime"`
	// Initial balance for this validator
	Balance uint64 `json:"balance"`
	// [Signer] is the BLS key for this validator.
	// Note: We do not enforce that the BLS key is unique across all validators.
	//       This means that validators can share a key if they so choose.
	//       However, a NodeID + Subnet does uniquely map to a BLS key
	Signer signer.Signer `json:"signer"`
	// Leftover $AVAX from the [Balance] will be issued to this
	// owner once it is removed from the validator set.
	ChangeOwner fx.Owner `json:"changeOwner"`
}

func CallDeploy(
	cmd *cobra.Command,
	subnetOnlyParam bool,
	blockchainName string,
	networkFlags networkoptions.NetworkFlags,
	keyNameParam string,
	useLedgerParam bool,
	useEwoqParam bool,
	sameControlKeyParam bool,
) error {
	subnetOnly = subnetOnlyParam
	globalNetworkFlags = networkFlags
	sameControlKey = sameControlKeyParam
	keyName = keyNameParam
	useLedger = useLedgerParam
	useEwoq = useEwoqParam
	return deployBlockchain(cmd, []string{blockchainName})
}

func getChainsInSubnet(blockchainName string) ([]string, error) {
	subnets, err := os.ReadDir(app.GetSubnetDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read baseDir: %w", err)
	}

	chains := []string{}

	for _, s := range subnets {
		if !s.IsDir() {
			continue
		}
		sidecarFile := filepath.Join(app.GetSubnetDir(), s.Name(), constants.SidecarFileName)
		if _, err := os.Stat(sidecarFile); err == nil {
			// read in sidecar file
			jsonBytes, err := os.ReadFile(sidecarFile)
			if err != nil {
				return nil, fmt.Errorf("failed reading file %s: %w", sidecarFile, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return nil, fmt.Errorf("failed unmarshaling file %s: %w", sidecarFile, err)
			}
			if sc.Subnet == blockchainName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

func checkSubnetEVMDefaultAddressNotInAlloc(network models.Network, chain string) error {
	if network.Kind != models.Local && network.Kind != models.Devnet && network.Kind != models.EtnaDevnet && os.Getenv(constants.SimulatePublicNetwork) == "" {
		genesis, err := app.LoadEvmGenesis(chain)
		if err != nil {
			return err
		}
		allocAddressMap := genesis.Alloc
		for address := range allocAddressMap {
			if address.String() == vm.PrefundedEwoqAddress.String() {
				return fmt.Errorf("can't airdrop to default address on public networks, please edit the genesis by calling `avalanche subnet create %s --force`", chain)
			}
		}
	}
	return nil
}

func runDeploy(cmd *cobra.Command, args []string, supportedNetworkOptions []networkoptions.NetworkOption) error {
	skipCreatePrompt = true
	deploySupportedNetworkOptions = supportedNetworkOptions
	return deployBlockchain(cmd, args)
}

func updateSubnetEVMGenesisChainID(genesisBytes []byte, newChainID uint) ([]byte, error) {
	var genesisMap map[string]interface{}
	if err := json.Unmarshal(genesisBytes, &genesisMap); err != nil {
		return nil, err
	}
	configI, ok := genesisMap["config"]
	if !ok {
		return nil, fmt.Errorf("config field not found on genesis")
	}
	config, ok := configI.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected genesis config field to be a map[string]interface, found %T", configI)
	}
	config["chainId"] = float64(newChainID)
	return json.MarshalIndent(genesisMap, "", "  ")
}

// updates sidecar with genesis mainnet id to use
// given either by cmdline flag, original genesis id, or id obtained from the user
func getSubnetEVMMainnetChainID(sc *models.Sidecar, blockchainName string) error {
	// get original chain id
	evmGenesis, err := app.LoadEvmGenesis(blockchainName)
	if err != nil {
		return err
	}
	if evmGenesis.Config == nil {
		return fmt.Errorf("invalid subnet evm genesis format: config is nil")
	}
	if evmGenesis.Config.ChainID == nil {
		return fmt.Errorf("invalid subnet evm genesis format: config chain id is nil")
	}
	originalChainID := evmGenesis.Config.ChainID.Uint64()
	// handle cmdline flag if given
	if mainnetChainID != 0 {
		sc.SubnetEVMMainnetChainID = uint(mainnetChainID)
	}
	// prompt the user
	if sc.SubnetEVMMainnetChainID == 0 {
		useSameChainID := "Use same ChainID"
		useNewChainID := "Use new ChainID"
		listOptions := []string{useNewChainID, useSameChainID}
		newChainIDPrompt := "Using the same ChainID for both Fuji and Mainnet could lead to a replay attack. Do you want to use a different ChainID?"
		var (
			err      error
			decision string
		)
		decision, err = app.Prompt.CaptureList(newChainIDPrompt, listOptions)
		if err != nil {
			return err
		}
		if decision == useSameChainID {
			sc.SubnetEVMMainnetChainID = uint(originalChainID)
		} else {
			ux.Logger.PrintToUser("Enter your subnet's ChainID. It can be any positive integer != %d.", originalChainID)
			newChainID, err := app.Prompt.CapturePositiveInt(
				"ChainID",
				[]prompts.Comparator{
					{
						Label: "Zero",
						Type:  prompts.MoreThan,
						Value: 0,
					},
					{
						Label: "Original Chain ID",
						Type:  prompts.NotEq,
						Value: originalChainID,
					},
				},
			)
			if err != nil {
				return err
			}
			sc.SubnetEVMMainnetChainID = uint(newChainID)
		}
	}
	return app.UpdateSidecar(sc)
}

// deployBlockchain is the cobra command run for deploying subnets
func deployBlockchain(cmd *cobra.Command, args []string) error {
	blockchainName := args[0]

	if err := CreateBlockchainFirst(cmd, blockchainName, skipCreatePrompt); err != nil {
		return err
	}

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	if icmSpec.MessengerContractAddressPath != "" || icmSpec.MessengerDeployerAddressPath != "" || icmSpec.MessengerDeployerTxPath != "" || icmSpec.RegistryBydecodePath != "" {
		if icmSpec.MessengerContractAddressPath == "" || icmSpec.MessengerDeployerAddressPath == "" || icmSpec.MessengerDeployerTxPath == "" || icmSpec.RegistryBydecodePath == "" {
			return fmt.Errorf("if setting any teleporter asset path, you must set all teleporter asset paths")
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
		return errors.New("unable to deploy subnets imported from a repo")
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	if !sidecar.Sovereign && bootstrapValidatorsJSONFilePath != "" {
		return fmt.Errorf("--bootstrap-filepath flag is only applicable to sovereign blockchains")
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		deploySupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

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

	ux.Logger.PrintToUser("Deploying %s to %s", chains, network.Name())

	if network.Kind == models.Local {
		app.Log.Debug("Deploy local")

		genesisPath := app.GetGenesisPath(chain)

		// copy vm binary to the expected location, first downloading it if necessary
		var vmBin string
		switch sidecar.VM {
		case models.SubnetEvm:
			_, vmBin, err = binutils.SetupSubnetEVM(app, sidecar.VMVersion)
			if err != nil {
				return fmt.Errorf("failed to install subnet-evm: %w", err)
			}
		case models.CustomVM:
			vmBin = binutils.SetupCustomBin(app, chain)
		default:
			return fmt.Errorf("unknown vm: %s", sidecar.VM)
		}

		// check if selected version matches what is currently running
		nc := localnet.NewStatusChecker()
		avagoVersion, err := CheckForInvalidDeployAndGetAvagoVersion(nc, sidecar.RPCVersion)
		if err != nil {
			return err
		}
		if avagoBinaryPath == "" {
			userProvidedAvagoVersion = avagoVersion
		}

		deployer := subnet.NewLocalDeployer(app, userProvidedAvagoVersion, avagoBinaryPath, vmBin)
		deployInfo, err := deployer.DeployToLocalNetwork(
			chain,
			genesisPath,
			icmSpec,
			subnetIDStr,
			constants.ServerRunFileLocalNetworkPrefix,
		)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(
					app,
					binutils.LocalNetworkGRPCServerEndpoint,
					constants.ServerRunFileLocalNetworkPrefix,
				); innerErr != nil {
					app.Log.Warn("tried to kill the gRPC server process but it failed", zap.Error(innerErr))
				}
			}
			return err
		}
		flags := make(map[string]string)
		flags[constants.MetricsNetwork] = network.Name()
		metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)
		if err := app.UpdateSidecarNetworks(
			&sidecar,
			network,
			deployInfo.SubnetID,
			deployInfo.BlockchainID,
			deployInfo.ICMMessengerAddress,
			deployInfo.ICMRegistryAddress,
			nil,
		); err != nil {
			return err
		}
		return PrintSubnetInfo(blockchainName, true)
	}

	if sidecar.Sovereign {
		if !generateNodeID {
			clusterName := fmt.Sprintf("%s-local-node", blockchainName)
			if globalNetworkFlags.ClusterName != "" {
				clusterName = globalNetworkFlags.ClusterName
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
								"please create your local node again and call subnet deploy command again", err)
						}
					}
					network = models.NewNetworkFromCluster(network, clusterName)
				}
			}
			// ask user if we want to use local machine if cluster is not provided
			if !useLocalMachine && globalNetworkFlags.ClusterName == "" {
				ux.Logger.PrintToUser("You can use your local machine as a bootstrap validator on the blockchain")
				ux.Logger.PrintToUser("This means that you don't have to to set up a remote server on a cloud service (e.g. AWS / GCP) to be a validator on the blockchain.")

				useLocalMachine, err = app.Prompt.CaptureYesNo("Do you want to use your local machine as a bootstrap validator?")
				if err != nil {
					return err
				}
			}
			// if no cluster provided - we create one  with fmt.Sprintf("%s-local-node", blockchainName) name
			if useLocalMachine && globalNetworkFlags.ClusterName == "" {
				// stop local avalanchego process so that we can generate new local cluster
				_ = node.StopLocalNode(app)
				anrSettings := node.ANRSettings{}
				avagoVersionSettings := node.AvalancheGoVersionSettings{}
				useEtnaDevnet := network.Kind == models.EtnaDevnet
				if avagoBinaryPath == "" {
					ux.Logger.PrintToUser("Local build of Avalanche Go is required to create an Avalanche node using local machine")
					ux.Logger.PrintToUser("Please download Avalanche Go repo at https://github.com/ava-labs/avalanchego and build from source through ./scripts/build.sh")
					ux.Logger.PrintToUser("Please provide the full path to Avalanche Go binary in the build directory (e.g, xxx/build/avalanchego)")
					avagoBinaryPath, err = app.Prompt.CaptureString("Path to Avalanche Go build")
					if err != nil {
						return err
					}
				}
				network = models.NewNetworkFromCluster(network, clusterName)
				nodeConfig := map[string]interface{}{}
				if app.AvagoNodeConfigExists(blockchainName) {
					nodeConfig, err = utils.ReadJSON(app.GetAvagoNodeConfigPath(blockchainName))
					if err != nil {
						return err
					}
				}
				nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
				// anrSettings, avagoVersionSettings, globalNetworkFlags are empty
				if err = node.StartLocalNode(
					app,
					clusterName,
					useEtnaDevnet,
					avagoBinaryPath,
					uint32(numLocalNodes),
					nodeConfig,
					anrSettings,
					avagoVersionSettings,
					globalNetworkFlags,
					nil,
				); err != nil {
					return err
				}
				if len(bootstrapEndpoints) == 0 {
					bootstrapEndpoints, err = getLocalBootstrapEndpoints()
					if err != nil {
						return fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
							"please create your local node again and call subnet deploy command again", err)
					}
				}
			}
		}
		switch {
		case len(bootstrapEndpoints) > 0:
			if changeOwnerAddress == "" {
				changeOwnerAddress, err = getKeyForChangeOwner(network)
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
					Balance:              constants.BootstrapValidatorBalance,
					BLSPublicKey:         publicKey,
					BLSProofOfPossession: pop,
					ChangeOwnerAddr:      changeOwnerAddress,
				})
			}
		case globalNetworkFlags.ClusterName != "":
			// for remote clusters we don't need to ask for bootstrap validators and can read it from filesystem
			bootstrapValidators, err = getClusterBootstrapValidators(globalNetworkFlags.ClusterName, network)
			if err != nil {
				return fmt.Errorf("error getting bootstrap validators from cluster %s: %w", globalNetworkFlags.ClusterName, err)
			}

		default:
			bootstrapValidators, err = promptBootstrapValidators(network, changeOwnerAddress, numBootstrapValidators)
			if err != nil {
				return err
			}
		}
	}

	// from here on we are assuming a public deploy
	if subnetOnly && subnetIDStr != "" {
		return errMutuallyExlusiveSubnetFlags
	}

	createSubnet := true
	var subnetID ids.ID
	if subnetIDStr != "" {
		subnetID, err = ids.FromString(subnetIDStr)
		if err != nil {
			return err
		}
		createSubnet = false
	} else if !subnetOnly && sidecar.Networks != nil {
		model, ok := sidecar.Networks[network.Name()]
		if ok {
			if model.SubnetID != ids.Empty && model.BlockchainID == ids.Empty {
				subnetID = model.SubnetID
				createSubnet = false
			}
		}
	}

	fee := uint64(0)
	if !subnetOnly {
		fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateBlockchainTxFee
	}
	if createSubnet {
		fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateSubnetTxFee
	}

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

	network.HandlePublicNetworkSimulation()

	if createSubnet {
		if sidecar.Sovereign {
			sameControlKey = true
		}
		controlKeys, threshold, err = promptOwners(
			kc,
			controlKeys,
			sameControlKey,
			threshold,
			subnetAuthKeys,
			true,
		)
		if err != nil {
			return err
		}
	} else {
		ux.Logger.PrintToUser(logging.Blue.Wrap(
			fmt.Sprintf("Deploying into pre-existent subnet ID %s", subnetID.String()),
		))
		var isPermissioned bool
		isPermissioned, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
		if err != nil {
			return err
		}
		if !isPermissioned {
			return ErrNotPermissionedSubnet
		}
	}

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
	ux.Logger.PrintToUser("Your subnet auth keys for chain creation: %s", subnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, kc, network)

	if createSubnet {
		subnetID, err = deployer.DeploySubnet(controlKeys, threshold)
		if err != nil {
			return err
		}
		deployer.CleanCacheWallet()
		// get the control keys in the same order as the tx
		_, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
		if err != nil {
			return err
		}
	}

	var (
		savePartialTx           bool
		blockchainID            ids.ID
		tx                      *txs.Tx
		remainingSubnetAuthKeys []string
		isFullySigned           bool
	)

	if !subnetOnly {
		isFullySigned, blockchainID, tx, remainingSubnetAuthKeys, err = deployer.DeployBlockchain(
			controlKeys,
			subnetAuthKeys,
			subnetID,
			chain,
			chainGenesis,
		)
		if err != nil {
			ux.Logger.PrintToUser(logging.Red.Wrap(
				fmt.Sprintf("error deploying blockchain: %s. fix the issue and try again with a new deploy cmd", err),
			))
		}

		savePartialTx = !isFullySigned && err == nil
	}

	if err := PrintDeployResults(chain, subnetID, blockchainID); err != nil {
		return err
	}

	if savePartialTx {
		if err := SaveNotFullySignedTx(
			"Blockchain Creation",
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

	if sidecar.Sovereign {
		avaGoBootstrapValidators, err := ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
		if err != nil {
			return err
		}
		deployer.CleanCacheWallet()
		managerAddress := common.HexToAddress(validatormanager.ValidatorContractAddress)
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
		ux.Logger.PrintToUser("ConvertL1Tx ID: %s", convertL1TxID)

		if savePartialTx {
			if err := SaveNotFullySignedTx(
				"ConvertL1Tx",
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
			"Waiting for L1 to be converted into sovereign blockchain ...",
			0,
		)
		if err != nil {
			return err
		}
		fmt.Println()

		if err := app.UpdateSidecarNetworks(&sidecar, network, subnetID, blockchainID, "", "", bootstrapValidators); err != nil {
			return err
		}

		if !convertOnly && !generateNodeID {
			clusterName := network.ClusterName
			if clusterName == "" {
				clusterName, err = node.GetClusterNameFromList(app)
				if err != nil {
					return err
				}
			}
			if !useLocalMachine {
				if err = node.SyncSubnet(app, clusterName, blockchainName, true, nil); err != nil {
					return err
				}

				if err := node.WaitForHealthyCluster(app, clusterName, node.HealthCheckTimeout, node.HealthCheckPoolTime); err != nil {
					return err
				}
			} else {
				if err := node.TrackSubnetWithLocalMachine(
					app,
					clusterName,
					blockchainName,
					avagoBinaryPath,
				); err != nil {
					return err
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
			extraAggregatorPeers, err := GetAggregatorExtraPeers(network, aggregatorExtraEndpoints)
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser("Initializing Proof of Authority Validator Manager contract on blockchain %s ...", blockchainName)
			subnetID, err := contract.GetSubnetID(
				app,
				network,
				chainSpec,
			)
			if err != nil {
				return err
			}
			blockchainID, err := contract.GetBlockchainID(
				app,
				network,
				chainSpec,
			)
			if err != nil {
				return err
			}
			ownerAddress := common.HexToAddress(sidecar.PoAValidatorManagerOwner)
			subnetSDK := blockchainSDK.Subnet{
				SubnetID:            subnetID,
				BlockchainID:        blockchainID,
				OwnerAddress:        &ownerAddress,
				RPC:                 rpcURL,
				BootstrapValidators: avaGoBootstrapValidators,
			}
			logLvl, err := logging.ToLevel(aggregatorLogLevel)
			if err != nil {
				logLvl = logging.Off
			}
			if err := subnetSDK.InitializeProofOfAuthority(network, genesisPrivateKey, extraAggregatorPeers, logLvl); err != nil {
				return err
			}
			ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
		} else {
			ux.Logger.GreenCheckmarkToUser("Converted subnet successfully generated")
			ux.Logger.PrintToUser("To finish conversion to sovereign L1, create the corresponding Avalanche node(s) with the provided Node ID and BLS Info")
			ux.Logger.PrintToUser("Created Node ID and BLS Info can be found at %s", app.GetSidecarPath(blockchainName))
			ux.Logger.PrintToUser("Once the Avalanche Node(s) are created and are tracking the blockchain, call `avalanche contract initPoaManager %s` to finish conversion to sovereign L1", blockchainName)
		}
	} else {
		if err := app.UpdateSidecarNetworks(&sidecar, network, subnetID, blockchainID, "", "", nil); err != nil {
			return err
		}
	}
	flags := make(map[string]string)
	flags[constants.MetricsNetwork] = network.Name()
	metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	return nil
}

func getClusterBootstrapValidators(clusterName string, network models.Network) ([]models.SubnetValidator, error) {
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	subnetValidators := []models.SubnetValidator{}
	hostIDs := utils.Filter(clusterConf.GetCloudIDs(), clusterConf.IsAvalancheGoHost)
	changeAddr := ""
	for _, h := range hostIDs {
		nodeID, pub, pop, err := utils.GetNodeParams(app.GetNodeInstanceDirPath(h))
		if err != nil {
			return nil, fmt.Errorf("failed to parse nodeID: %w", err)
		}
		changeAddr, err = getKeyForChangeOwner(network)
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		ux.Logger.Info("Bootstrap validator info for Host: %s | Node ID: %s | Public Key: %s | Proof of Possession: %s", h, nodeID, hex.EncodeToString(pub), hex.EncodeToString(pop))
		subnetValidators = append(subnetValidators, models.SubnetValidator{
			NodeID:               nodeID.String(),
			Weight:               constants.BootstrapValidatorWeight,
			Balance:              constants.BootstrapValidatorBalance,
			BLSPublicKey:         fmt.Sprintf("%s%s", "0x", hex.EncodeToString(pub)),
			BLSProofOfPossession: fmt.Sprintf("%s%s", "0x", hex.EncodeToString(pop)),
			ChangeOwnerAddr:      changeAddr,
		})
	}
	return subnetValidators, nil
}

func getBLSInfo(publicKey, proofOfPossesion string) (signer.ProofOfPossession, error) {
	type jsonProofOfPossession struct {
		PublicKey         string
		ProofOfPossession string
	}
	jsonPop := jsonProofOfPossession{
		PublicKey:         publicKey,
		ProofOfPossession: proofOfPossesion,
	}
	popBytes, err := json.Marshal(jsonPop)
	if err != nil {
		return signer.ProofOfPossession{}, err
	}
	pop := &signer.ProofOfPossession{}
	err = pop.UnmarshalJSON(popBytes)
	if err != nil {
		return signer.ProofOfPossession{}, err
	}
	return *pop, nil
}

// TODO: add deactivation owner?
func ConvertToAvalancheGoSubnetValidator(subnetValidators []models.SubnetValidator) ([]*txs.ConvertSubnetValidator, error) {
	bootstrapValidators := []*txs.ConvertSubnetValidator{}
	for _, validator := range subnetValidators {
		nodeID, err := ids.NodeIDFromString(validator.NodeID)
		if err != nil {
			return nil, err
		}
		blsInfo, err := getBLSInfo(validator.BLSPublicKey, validator.BLSProofOfPossession)
		if err != nil {
			return nil, fmt.Errorf("failure parsing BLS info: %w", err)
		}
		addrs, err := address.ParseToIDs([]string{validator.ChangeOwnerAddr})
		if err != nil {
			return nil, fmt.Errorf("failure parsing change owner address: %w", err)
		}
		bootstrapValidator := &txs.ConvertSubnetValidator{
			NodeID:  nodeID[:],
			Weight:  validator.Weight,
			Balance: validator.Balance,
			Signer:  blsInfo,
			RemainingBalanceOwner: message.PChainOwner{
				Threshold: 1,
				Addresses: addrs,
			},
		}
		bootstrapValidators = append(bootstrapValidators, bootstrapValidator)
	}
	avagoutils.Sort(bootstrapValidators)
	return bootstrapValidators, nil
}

func ValidateSubnetNameAndGetChains(args []string) ([]string, error) {
	// this should not be necessary but some bright guy might just be creating
	// the genesis by hand or something...
	if err := checkInvalidSubnetNames(args[0]); err != nil {
		return nil, fmt.Errorf("subnet name %s is invalid: %w", args[0], err)
	}
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return nil, errors.New("Invalid subnet " + args[0])
	}

	return chains, nil
}

func SaveNotFullySignedTx(
	txName string,
	tx *txs.Tx,
	chain string,
	subnetAuthKeys []string,
	remainingSubnetAuthKeys []string,
	outputTxPath string,
	forceOverwrite bool,
) error {
	signedCount := len(subnetAuthKeys) - len(remainingSubnetAuthKeys)
	ux.Logger.PrintToUser("")
	if signedCount == len(subnetAuthKeys) {
		ux.Logger.PrintToUser("All %d required %s signatures have been signed. "+
			"Saving tx to disk to enable commit.", len(subnetAuthKeys), txName)
	} else {
		ux.Logger.PrintToUser("%d of %d required %s signatures have been signed. "+
			"Saving tx to disk to enable remaining signing.", signedCount, len(subnetAuthKeys), txName)
	}
	if outputTxPath == "" {
		ux.Logger.PrintToUser("")
		var err error
		if forceOverwrite {
			outputTxPath, err = app.Prompt.CaptureString("Path to export partially signed tx to")
		} else {
			outputTxPath, err = app.Prompt.CaptureNewFilepath("Path to export partially signed tx to")
		}
		if err != nil {
			return err
		}
	}
	if forceOverwrite {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Overwriting %s", outputTxPath)
	}
	if err := txutils.SaveToDisk(tx, outputTxPath, forceOverwrite); err != nil {
		return err
	}
	if signedCount == len(subnetAuthKeys) {
		PrintReadyToSignMsg(chain, outputTxPath)
	} else {
		PrintRemainingToSignMsg(chain, remainingSubnetAuthKeys, outputTxPath)
	}
	return nil
}

func PrintReadyToSignMsg(
	chain string,
	outputTxPath string,
) {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Tx is fully signed, and ready to be committed")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Commit command:")
	ux.Logger.PrintToUser("  avalanche transaction commit %s --input-tx-filepath %s", chain, outputTxPath)
}

func PrintRemainingToSignMsg(
	chain string,
	remainingSubnetAuthKeys []string,
	outputTxPath string,
) {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Addresses remaining to sign the tx")
	for _, subnetAuthKey := range remainingSubnetAuthKeys {
		ux.Logger.PrintToUser("  %s", subnetAuthKey)
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Connect a ledger with one of the remaining addresses or choose a stored key "+
		"and run the signing command, or send %q to another user for signing.", outputTxPath)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Signing command:")
	ux.Logger.PrintToUser("  avalanche transaction sign %s --input-tx-filepath %s", chain, outputTxPath)
	ux.Logger.PrintToUser("")
}

func PrintDeployResults(chain string, subnetID ids.ID, blockchainID ids.ID) error {
	vmID, err := anrutils.VMID(chain)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	header := []string{"Deployment results", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"VM ID", vmID.String()})
	if blockchainID != ids.Empty {
		table.Append([]string{"Blockchain ID", blockchainID.String()})
		table.Append([]string{"P-Chain TXID", blockchainID.String()})
	}
	table.Render()
	return nil
}

// Determines the appropriate version of avalanchego to run with. Returns an error if
// that version conflicts with the current deployment.
func CheckForInvalidDeployAndGetAvagoVersion(
	statusChecker localnet.StatusChecker,
	configuredRPCVersion int,
) (string, error) {
	// get current network
	runningAvagoVersion, runningRPCVersion, networkRunning, err := statusChecker.GetCurrentNetworkVersion()
	if err != nil {
		return "", err
	}
	desiredAvagoVersion := userProvidedAvagoVersion

	// RPC Version was made available in the info API in avalanchego version v1.9.2. For prior versions,
	// we will need to skip this check.
	skipRPCCheck := false
	if semver.Compare(runningAvagoVersion, constants.AvalancheGoCompatibilityVersionAdded) == -1 {
		skipRPCCheck = true
	}

	if networkRunning {
		if userProvidedAvagoVersion == "latest" {
			if runningRPCVersion != configuredRPCVersion && !skipRPCCheck {
				return "", fmt.Errorf(
					"the current avalanchego deployment uses rpc version %d but your subnet has version %d and is not compatible",
					runningRPCVersion,
					configuredRPCVersion,
				)
			}
			desiredAvagoVersion = runningAvagoVersion
		} else if runningAvagoVersion != strings.Split(userProvidedAvagoVersion, "-")[0] {
			// user wants a specific version
			return "", errors.New("incompatible avalanchego version selected")
		}
	} else if userProvidedAvagoVersion == "latest" {
		// find latest avago version for this rpc version
		desiredAvagoVersion, err = vm.GetLatestAvalancheGoByProtocolVersion(
			app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
		if err == vm.ErrNoAvagoVersion {
			latestPreReleaseVersion, err := app.Downloader.GetLatestPreReleaseVersion(
				constants.AvaLabsOrg,
				constants.AvalancheGoRepoName,
			)
			if err != nil {
				return "", err
			}
			return latestPreReleaseVersion, nil
		}
		if err != nil {
			return "", err
		}
	}
	return desiredAvagoVersion, nil
}

func LoadBootstrapValidator(filepath string) ([]models.SubnetValidator, error) {
	if !utils.FileExists(filepath) {
		return nil, fmt.Errorf("file path %q doesn't exist", filepath)
	}
	jsonBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	var subnetValidators []models.SubnetValidator
	if err = json.Unmarshal(jsonBytes, &subnetValidators); err != nil {
		return nil, err
	}
	if err = validateSubnetValidatorsJSON(generateNodeID, subnetValidators); err != nil {
		return nil, err
	}
	if generateNodeID {
		for _, subnetValidator := range subnetValidators {
			subnetValidator.NodeID, subnetValidator.BLSPublicKey, subnetValidator.BLSProofOfPossession, err = generateNewNodeAndBLS()
			if err != nil {
				return nil, err
			}
		}
	}
	return subnetValidators, nil
}

func UrisToPeers(uris []string) ([]info.Peer, error) {
	peers := []info.Peer{}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	for _, uri := range uris {
		client := info.NewClient(uri)
		nodeID, _, err := client.GetNodeID(ctx)
		if err != nil {
			return nil, err
		}
		ip, err := client.GetNodeIP(ctx)
		if err != nil {
			return nil, err
		}
		peers = append(peers, info.Peer{
			Info: peer.Info{
				ID:       nodeID,
				PublicIP: ip,
			},
		})
	}
	return peers, nil
}

func ConvertURIToPeers(uris []string) ([]info.Peer, error) {
	aggregatorPeers, err := UrisToPeers(uris)
	if err != nil {
		return nil, err
	}
	nodeIDs := utils.Map(aggregatorPeers, func(peer info.Peer) ids.NodeID {
		return peer.Info.ID
	})
	nodeIDsSet := set.Of(nodeIDs...)
	for _, uri := range uris {
		infoClient := info.NewClient(uri)
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		peers, err := infoClient.Peers(ctx)
		if err != nil {
			return nil, err
		}
		for _, peer := range peers {
			if !nodeIDsSet.Contains(peer.Info.ID) {
				aggregatorPeers = append(aggregatorPeers, peer)
				nodeIDsSet.Add(peer.Info.ID)
			}
		}
	}
	return aggregatorPeers, nil
}

func GetAggregatorExtraPeers(
	network models.Network,
	extraURIs []string,
) ([]info.Peer, error) {
	uris, err := GetAggregatorNetworkUris(network)
	if err != nil {
		return nil, err
	}
	uris = append(uris, extraURIs...)
	urisSet := set.Of(uris...)
	uris = urisSet.List()
	return ConvertURIToPeers(uris)
}

func GetAggregatorNetworkUris(network models.Network) ([]string, error) {
	aggregatorExtraPeerEndpointsUris := []string{}
	if network.ClusterName != "" {
		clustersConfig, err := app.LoadClustersConfig()
		if err != nil {
			return nil, err
		}
		clusterConfig := clustersConfig.Clusters[network.ClusterName]
		if clusterConfig.Local {
			cli, err := binutils.NewGRPCClientWithEndpoint(
				binutils.LocalClusterGRPCServerEndpoint,
				binutils.WithAvoidRPCVersionCheck(true),
				binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
			)
			if err != nil {
				return nil, err
			}
			ctx, cancel := utils.GetANRContext()
			defer cancel()
			status, err := cli.Status(ctx)
			if err != nil {
				return nil, err
			}
			for _, nodeInfo := range status.ClusterInfo.NodeInfos {
				aggregatorExtraPeerEndpointsUris = append(aggregatorExtraPeerEndpointsUris, nodeInfo.Uri)
			}
		} else { // remote cluster case
			hostIDs := utils.Filter(clusterConfig.GetCloudIDs(), clusterConfig.IsAvalancheGoHost)
			for _, hostID := range hostIDs {
				if nodeConfig, err := app.LoadClusterNodeConfig(hostID); err != nil {
					return nil, err
				} else {
					aggregatorExtraPeerEndpointsUris = append(aggregatorExtraPeerEndpointsUris, fmt.Sprintf("http://%s:%d", nodeConfig.ElasticIP, constants.AvalanchegoAPIPort))
				}
			}
		}
	}
	return aggregatorExtraPeerEndpointsUris, nil
}

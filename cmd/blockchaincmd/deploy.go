// / Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/network/peer"

	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/relayercmd"
	"github.com/ava-labs/avalanche-cli/cmd/networkcmd"
	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	avagoutils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const skipRelayerFlagName = "skip-relayer"

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
	partialSync                     bool
	changeOwnerAddress              string
	subnetOnly                      bool
	icmSpec                         subnet.ICMSpec
	generateNodeID                  bool
	bootstrapValidatorsJSONFilePath string
	privateKeyFlags                 contract.PrivateKeyFlags
	bootstrapEndpoints              []string
	convertOnly                     bool
	numNodes                        uint32
	relayerAmount                   float64
	relayerKeyName                  string
	relayCChain                     bool
	cChainFundingKey                string
	icmKeyName                      string
	cchainIcmKeyName                string

	poSMinimumStakeAmount     uint64
	poSMaximumStakeAmount     uint64
	poSMinimumStakeDuration   uint64
	poSMinimumDelegationFee   uint16
	poSMaximumStakeMultiplier uint8
	poSWeightToValueFactor    uint64

	errMutuallyExlusiveControlKeys = errors.New("--control-keys and --same-control-key are mutually exclusive")
	ErrMutuallyExlusiveKeyLedger   = errors.New("key source flags --key, --ledger/--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet          = errors.New("key --key is not available for mainnet operations")
	errMutuallyExlusiveSubnetFlags = errors.New("--subnet-only and --subnet-id are mutually exclusive")
	errNotSupportedOnMainnet       = errors.New("deploying sovereign blockchain is currently not supported on Mainnet")
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
	cmd.Flags().StringVar(
		&userProvidedAvagoVersion,
		"avalanchego-version",
		constants.DefaultAvalancheGoVersion,
		"use this version of avalanchego (ex: v1.17.12)",
	)
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
	cmd.Flags().BoolVar(&icmSpec.SkipRelayerDeploy, skipRelayerFlagName, false, "skip relayer deploy")
	cmd.Flags().StringVar(
		&icmSpec.ICMVersion,
		"teleporter-version",
		constants.LatestReleaseVersionTag,
		"teleporter version to deploy",
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
	cmd.Flags().BoolVar(&aggregatorAllowPrivatePeers, "aggregator-allow-private-peers", true, "allow the signature aggregator to connect to peers with private IP")
	cmd.Flags().BoolVar(&useLocalMachine, "use-local-machine", false, "use local machine as a blockchain validator")
	cmd.Flags().IntVar(&numBootstrapValidators, "num-bootstrap-validators", 0, "(only if --generate-node-id is true) number of bootstrap validators to set up in sovereign L1 validator)")
	cmd.Flags().IntVar(&numLocalNodes, "num-local-nodes", 0, "number of nodes to be created on local machine")
	cmd.Flags().StringVar(&changeOwnerAddress, "change-owner-address", "", "address that will receive change if node is no longer L1 validator")

	cmd.Flags().Uint64Var(&poSMinimumStakeAmount, "pos-minimum-stake-amount", 1, "minimum stake amount")
	cmd.Flags().Uint64Var(&poSMaximumStakeAmount, "pos-maximum-stake-amount", 1000, "maximum stake amount")
	cmd.Flags().Uint64Var(&poSMinimumStakeDuration, "pos-minimum-stake-duration", 100, "minimum stake duration")
	cmd.Flags().Uint16Var(&poSMinimumDelegationFee, "pos-minimum-delegation-fee", 1, "minimum delegation fee")
	cmd.Flags().Uint8Var(&poSMaximumStakeMultiplier, "pos-maximum-stake-multiplier", 1, "maximum stake multiplier")
	cmd.Flags().Uint64Var(&poSWeightToValueFactor, "pos-weight-to-value-factor", 1, "weight to value factor")

	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "set primary network partial sync for new validators")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", constants.LocalNetworkNumNodes, "number of nodes to be created on local network deploy")
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
	if network.Kind != models.Local &&
		network.Kind != models.Devnet &&
		network.Kind != models.EtnaDevnet && !simulatedPublicNetwork() {
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
	clusterNameFlagValue = globalNetworkFlags.ClusterName

	if !simulatedPublicNetwork() {
		if network.Kind == models.Mainnet && sidecar.Sovereign {
			return errNotSupportedOnMainnet
		}
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

		avagoVersion := userProvidedAvagoVersion

		if avagoVersion == constants.DefaultAvalancheGoVersion && avagoBinaryPath == "" {
			// nothing given: get avago version from RPC compat
			avagoVersion, err = vm.GetLatestAvalancheGoByProtocolVersion(
				app,
				sidecar.RPCVersion,
				constants.AvalancheGoCompatibilityURL,
			)
			if err != nil {
				if err != vm.ErrNoAvagoVersion {
					return err
				}
				avagoVersion = constants.LatestPreReleaseVersionTag
			}
			// TODO: remove after etna release is available
			if sidecar.RPCVersion == constants.FirstEtnaRPCVersion {
				avagoVersion = constants.LatestPreReleaseVersionTag
			}
		}

		ux.Logger.PrintToUser("")
		if err := networkcmd.Start(
			networkcmd.StartFlags{
				UserProvidedAvagoVersion: avagoVersion,
				AvagoBinaryPath:          avagoBinaryPath,
				NumNodes:                 numNodes,
			},
			false,
		); err != nil {
			return err
		}

		// check if blockchain rpc version matches what is currently running
		// for the case version or binary was provided
		_, _, networkRPCVersion, err := localnet.GetVersion()
		if err != nil {
			return err
		}
		if networkRPCVersion != sidecar.RPCVersion {
			return fmt.Errorf(
				"the current local network uses rpc version %d but your blockchain has version %d and is not compatible",
				networkRPCVersion,
				sidecar.RPCVersion,
			)
		}

		useEwoq = true

		if b, err := networkcmd.AlreadyDeployed(blockchainName); err != nil {
			return err
		} else if b {
			return fmt.Errorf("blockchain %s has already been deployed", blockchainName)
		}
	}
	// end of local deploy

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
	//if !subnetOnly {
	//	fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateBlockchainTxFee
	//}
	//if createSubnet {
	//	fee += network.GenesisParams().TxFeeConfig.StaticFeeConfig.CreateSubnetTxFee
	//}

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

	if sidecar.Sovereign {
		if changeOwnerAddress == "" {
			// use provided key as change owner unless already set
			if pAddr, err := kc.PChainFormattedStrAddresses(); err == nil && len(pAddr) > 0 {
				changeOwnerAddress = pAddr[0]
				ux.Logger.PrintToUser("Using [%s] to be set as a change owner for leftover AVAX", changeOwnerAddress)
			}
		}
		if !generateNodeID {
			if network.Kind == models.Local {
				if len(bootstrapEndpoints) == 0 {
					bootstrapEndpoints, err = getLocalBootstrapEndpoints(network)
					if err != nil {
						return fmt.Errorf("error getting local host bootstrap endpoints: %w", err)
					}
				}
				if changeOwnerAddress == "" {
					k, err := app.GetKey("ewoq", network, false)
					if err != nil {
						return err
					}
					changeOwnerAddress = k.P()[0]
				}
			}
			clusterName := fmt.Sprintf("%s-local-node", blockchainName)
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
						bootstrapEndpoints, err = getLocalBootstrapEndpoints(network)
						if err != nil {
							return fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
								"please create your local node again and call subnet deploy command again", err)
						}
					}
					network = models.ConvertClusterToNetwork(network)
				}
			}
			if numLocalNodes > 0 {
				useLocalMachine = true
			}
			// ask user if we want to use local machine if cluster is not provided
			if network.Kind != models.Local && !useLocalMachine && clusterNameFlagValue == "" {
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
			// if no cluster provided - we create one  with fmt.Sprintf("%s-local-node", blockchainName) name
			if useLocalMachine && clusterNameFlagValue == "" {
				// stop local avalanchego process so that we can generate new local cluster
				_ = node.StopLocalNode(app)
				anrSettings := node.ANRSettings{}
				avagoVersionSettings := node.AvalancheGoVersionSettings{}
				useEtnaDevnet := network.Kind == models.EtnaDevnet
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
				clusterNameFlagValue = clusterName
				if len(bootstrapEndpoints) == 0 {
					bootstrapEndpoints, err = getLocalBootstrapEndpoints(network)
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
		case clusterNameFlagValue != "":
			// for remote clusters we don't need to ask for bootstrap validators and can read it from filesystem
			bootstrapValidators, err = getClusterBootstrapValidators(clusterNameFlagValue, network)
			if err != nil {
				return fmt.Errorf("error getting bootstrap validators from cluster %s: %w", clusterNameFlagValue, err)
			}

		default:
			bootstrapValidators, err = promptBootstrapValidators(network, changeOwnerAddress, numBootstrapValidators)
			if err != nil {
				return err
			}
		}
	} else if network.Kind == models.Local {
		sameControlKey = true
	}

	// from here on we are assuming a public deploy
	if subnetOnly && subnetIDStr != "" {
		return errMutuallyExlusiveSubnetFlags
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

	//if createSubnet {
	//	subnetID, err = deployer.DeploySubnet(controlKeys, threshold)
	//	if err != nil {
	//		return err
	//	}
	//	deployer.CleanCacheWallet()
	//	// get the control keys in the same order as the tx
	//	_, controlKeys, threshold, err = txutils.GetOwners(network, subnetID)
	//	if err != nil {
	//		return err
	//	}
	//}

	//var (
	//	savePartialTx           bool
	//	blockchainID            ids.ID
	//	tx                      *txs.Tx
	//	remainingSubnetAuthKeys []string
	//	isFullySigned           bool
	//)

	//if !subnetOnly {
	//	isFullySigned, blockchainID, tx, remainingSubnetAuthKeys, err = deployer.DeployBlockchain(
	//		controlKeys,
	//		subnetAuthKeys,
	//		subnetID,
	//		chain,
	//		chainGenesis,
	//	)
	//	if err != nil {
	//		ux.Logger.PrintToUser(logging.Red.Wrap(
	//			fmt.Sprintf("error deploying blockchain: %s. fix the issue and try again with a new deploy cmd", err),
	//		))
	//	}
	//
	//	savePartialTx = !isFullySigned && err == nil
	//}
	//
	//if err := PrintDeployResults(chain, subnetID, blockchainID); err != nil {
	//	return err
	//}

	//if savePartialTx {
	//	if err := SaveNotFullySignedTx(
	//		"Blockchain Creation",
	//		tx,
	//		chain,
	//		subnetAuthKeys,
	//		remainingSubnetAuthKeys,
	//		outputTxPath,
	//		false,
	//	); err != nil {
	//		return err
	//	}
	//}

	tracked := false

	subnetID, err = ids.FromString("fRfQ6Jak6gPSYPdUgDY8pgEgaLHcMAxMFArghATJUYivAvUqj")
	if err != nil {
		return err
	}
	blockchainID, err := ids.FromString("2i4YaQ8SJpBwF7M9rP18SK5xUSytUnpEjmSo9zhbDxTmdSQ79t")
	if err != nil {
		return err
	}
	if sidecar.Sovereign {
		avaGoBootstrapValidators, err := ConvertToAvalancheGoSubnetValidator(bootstrapValidators)
		if err != nil {
			return err
		}
		deployer.CleanCacheWallet()
		managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
		_, convertL1TxID, _, _, err := deployer.ConvertL1(
			controlKeys,
			subnetAuthKeys,
			subnetID,
			blockchainID,
			managerAddress,
			avaGoBootstrapValidators,
		)
		if err != nil {
			ux.Logger.RedXToUser("error converting subnet: %s. fix the issue and try again with a new convert cmd", err)
			return err
		}

		//savePartialTx = !isFullySigned && err == nil
		ux.Logger.PrintToUser("ConvertSubnetToL1Tx ID: %s", convertL1TxID)

		//if savePartialTx {
		//	if err := SaveNotFullySignedTx(
		//		"ConvertSubnetToL1Tx",
		//		tx,
		//		chain,
		//		subnetAuthKeys,
		//		remainingSubnetAuthKeys,
		//		outputTxPath,
		//		false,
		//	); err != nil {
		//		return err
		//	}
		//}

		_, err = ux.TimedProgressBar(
			30*time.Second,
			"Waiting for L1 to be converted into sovereign blockchain ...",
			0,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("")

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
			if network.Kind != models.Local {
				if clusterName == "" {
					clusterName, err = node.GetClusterNameFromList(app)
					if err != nil {
						return err
					}
				}
			}
			switch {
			case network.Kind == models.Local:
				if err := networkcmd.TrackSubnet(
					blockchainName,
					avagoBinaryPath,
					sidecar.Sovereign,
				); err != nil {
					return err
				}
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
				if err = node.SyncSubnet(app, clusterName, blockchainName, true, nil); err != nil {
					return err
				}

				if err := node.WaitForHealthyCluster(app, clusterName, node.HealthCheckTimeout, node.HealthCheckPoolTime); err != nil {
					return err
				}
			}
			tracked = true
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
			extraAggregatorPeers, err := GetAggregatorExtraPeers(clusterName, aggregatorExtraEndpoints)
			if err != nil {
				return err
			}
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
			ownerAddress := common.HexToAddress(sidecar.ValidatorManagerOwner)
			fmt.Printf("ownerAddress %s \n", ownerAddress.String())
			fmt.Printf("subnetID %s \n", subnetID.String())

			fmt.Printf("blockchainID %s \n", blockchainID.String())
			fmt.Printf("rpcURL %s \n", rpcURL)

			fmt.Printf("extraAggregatorPeers %s \n", extraAggregatorPeers)

			fmt.Printf("avaGoBootstrapValidators %s \n", avaGoBootstrapValidators)
			fmt.Printf("genesisPrivateKey %s \n", genesisPrivateKey)

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
			if sidecar.ValidatorManagement == models.ProofOfStake {
				ux.Logger.PrintToUser("Initializing Native Token Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
				if err := subnetSDK.InitializeProofOfStake(
					network,
					genesisPrivateKey,
					extraAggregatorPeers,
					aggregatorAllowPrivatePeers,
					logLvl,
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
					logLvl,
				); err != nil {
					return err
				}
				ux.Logger.GreenCheckmarkToUser("Proof of Authority Validator Manager contract successfully initialized on blockchain %s", blockchainName)
			}
		} else {
			ux.Logger.GreenCheckmarkToUser("Converted subnet successfully generated")
			ux.Logger.PrintToUser("To finish conversion to sovereign L1, create the corresponding Avalanche node(s) with the provided Node ID and BLS Info")
			ux.Logger.PrintToUser("Created Node ID and BLS Info can be found at %s", app.GetSidecarPath(blockchainName))
			ux.Logger.PrintToUser("Once the Avalanche Node(s) are created and are tracking the blockchain, call `avalanche contract initValidatorManager %s` to finish conversion to sovereign L1", blockchainName)
		}
	} else {
		if err := app.UpdateSidecarNetworks(
			&sidecar,
			network,
			subnetID,
			blockchainID,
			"",
			"",
			nil,
			clusterNameFlagValue,
		); err != nil {
			return err
		}
		if network.Kind == models.Local && !simulatedPublicNetwork() {
			ux.Logger.PrintToUser("")
			if err := networkcmd.TrackSubnet(
				blockchainName,
				avagoBinaryPath,
				sidecar.Sovereign,
			); err != nil {
				return err
			}
			tracked = true
		}
	}

	if sidecar.TeleporterReady && tracked {
		if !icmSpec.SkipICMDeploy {
			chainSpec := contract.ChainSpec{
				BlockchainName: blockchainName,
			}
			chainSpec.SetEnabled(true, false, false, false, false)
			deployICMFlags := teleportercmd.DeployFlags{
				ChainFlags: chainSpec,
				PrivateKeyFlags: contract.PrivateKeyFlags{
					KeyName: icmKeyName,
				},
				DeployMessenger:              true,
				DeployRegistry:               true,
				ForceRegistryDeploy:          true,
				Version:                      icmSpec.ICMVersion,
				MessengerContractAddressPath: icmSpec.MessengerContractAddressPath,
				MessengerDeployerAddressPath: icmSpec.MessengerDeployerAddressPath,
				MessengerDeployerTxPath:      icmSpec.MessengerDeployerTxPath,
				RegistryBydecodePath:         icmSpec.RegistryBydecodePath,
				CChainKeyName:                cchainIcmKeyName,
			}
			ux.Logger.PrintToUser("")
			if err := teleportercmd.CallDeploy([]string{}, deployICMFlags, network); err != nil {
				return err
			}
		}
		if network.Kind != models.Local && !useLocalMachine {
			if flag := cmd.Flags().Lookup(skipRelayerFlagName); flag != nil && !flag.Changed {
				ux.Logger.PrintToUser("")
				yes, err := app.Prompt.CaptureYesNo("Do you want to use set up a local interchain relayer?")
				if err != nil {
					return err
				}
				icmSpec.SkipRelayerDeploy = !yes
			}
		}
		if !icmSpec.SkipRelayerDeploy && (network.Kind != models.Fuji && network.Kind != models.Mainnet) {
			deployRelayerFlags := relayercmd.DeployFlags{
				Version:            icmSpec.RelayerVersion,
				BinPath:            icmSpec.RelayerBinPath,
				LogLevel:           icmSpec.RelayerLogLevel,
				RelayCChain:        relayCChain,
				CChainFundingKey:   cChainFundingKey,
				BlockchainsToRelay: []string{blockchainName},
				Key:                relayerKeyName,
				Amount:             relayerAmount,
			}
			if network.Kind == models.Local || useLocalMachine {
				deployRelayerFlags.Key = constants.ICMRelayerKeyName
				deployRelayerFlags.Amount = constants.DefaultRelayerAmount
				deployRelayerFlags.BlockchainFundingKey = constants.ICMKeyName
			}
			if network.Kind == models.Local {
				deployRelayerFlags.CChainFundingKey = "ewoq"
			}
			if err := relayercmd.CallDeploy(nil, deployRelayerFlags, network); err != nil {
				return err
			}
		}
	}

	flags := make(map[string]string)
	flags[constants.MetricsNetwork] = network.Name()
	metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)

	if network.Kind == models.Local && !simulatedPublicNetwork() {
		ux.Logger.PrintToUser("")
		return PrintSubnetInfo(blockchainName, true)
	}

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
func ConvertToAvalancheGoSubnetValidator(subnetValidators []models.SubnetValidator) ([]*txs.ConvertSubnetToL1Validator, error) {
	bootstrapValidators := []*txs.ConvertSubnetToL1Validator{}
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
		bootstrapValidator := &txs.ConvertSubnetToL1Validator{
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
		peers, err := infoClient.Peers(ctx, nil)
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
	clusterName string,
	extraURIs []string,
) ([]info.Peer, error) {
	uris, err := GetAggregatorNetworkUris(clusterName)
	if err != nil {
		return nil, err
	}
	uris = append(uris, extraURIs...)
	urisSet := set.Of(uris...)
	uris = urisSet.List()
	return UrisToPeers(uris)
}

func GetAggregatorNetworkUris(clusterName string) ([]string, error) {
	aggregatorExtraPeerEndpointsUris := []string{}
	if clusterName != "" {
		clustersConfig, err := app.LoadClustersConfig()
		if err != nil {
			return nil, err
		}
		clusterConfig := clustersConfig.Clusters[clusterName]
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
					aggregatorExtraPeerEndpointsUris = append(aggregatorExtraPeerEndpointsUris, fmt.Sprintf("http://%s:%d", nodeConfig.ElasticIP, constants.AvalancheGoAPIPort))
				}
			}
		}
	}
	return aggregatorExtraPeerEndpointsUris, nil
}

func simulatedPublicNetwork() bool {
	return os.Getenv(constants.SimulatePublicNetwork) != ""
}

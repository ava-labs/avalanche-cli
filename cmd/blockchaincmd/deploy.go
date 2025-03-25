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

	"github.com/ava-labs/avalanche-cli/cmd/flags"

	"golang.org/x/mod/semver"

	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"

	"github.com/ava-labs/avalanche-cli/pkg/blockchain"

	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/messengercmd"
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/relayercmd"
	"github.com/ava-labs/avalanche-cli/cmd/networkcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoutils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/message"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

const skipRelayerFlagName = "skip-relayer"

var (
	keyName                         string
	userProvidedAvagoVersion        string
	useLedger                       bool
	useLocalMachine                 bool
	useEwoq                         bool
	ledgerAddresses                 []string
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
	bootstrapEndpoints              []string
	convertOnly                     bool
	numNodes                        uint32
	relayerAmount                   float64
	relayerKeyName                  string
	relayCChain                     bool
	cChainFundingKey                string
	icmKeyName                      string
	cchainIcmKeyName                string
	relayerAllowPrivateIPs          bool

	poSMinimumStakeAmount          uint64
	poSMaximumStakeAmount          uint64
	poSMinimumStakeDuration        uint64
	poSMinimumDelegationFee        uint16
	poSMaximumStakeMultiplier      uint8
	poSWeightToValueFactor         uint64
	deployBalanceAVAX              float64
	validatorManagerAddress        string
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
so you can take your locally tested Blockchain and deploy it on Fuji or Mainnet.`,
		RunE:              deployBlockchain,
		PersistentPostRun: handlePostRun,
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	flags.AddSignatureAggregatorFlagsToCmd(cmd)
	cmd.Flags().StringVar(
		&userProvidedAvagoVersion,
		"avalanchego-version",
		constants.DefaultAvalancheGoVersion,
		"use this version of avalanchego (ex: v1.17.12)",
	)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet deploy only]")
	flags.AddNonSovFlagsToCmd(cmd, true)
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
) error {
	subnetOnly = subnetOnlyParam
	globalNetworkFlags = networkFlags
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
		!simulatedPublicNetwork() {
		genesis, err := app.LoadEvmGenesis(chain)
		if err != nil {
			return err
		}
		allocAddressMap := genesis.Alloc
		for address := range allocAddressMap {
			if address.String() == vm.PrefundedEwoqAddress.String() {
				return fmt.Errorf("can't airdrop to default address on public networks, please edit the genesis by calling `avalanche blockchain create %s --force`", chain)
			}
		}
	}
	return nil
}

func runDeploy(cmd *cobra.Command, args []string) error {
	skipCreatePrompt = true
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
			ux.Logger.PrintToUser("Enter your blockchain's ChainID. It can be any positive integer != %d.", originalChainID)
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

	// TODO: can we actually just do this on global level on flag/blockchain.go?
	if err = flags.NonSovFlags.ValidateOutputTxPath(); err != nil {
		return err
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
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	clusterNameFlagValue = globalNetworkFlags.ClusterName

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

	if network.Kind == models.Fuji && userProvidedAvagoVersion == constants.DefaultAvalancheGoVersion {
		latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(
			constants.AvaLabsOrg,
			constants.AvalancheGoRepoName,
			"",
		)
		if err != nil {
			return err
		}
		versionComparison := semver.Compare(constants.FujiAvalancheGoV113, latestAvagoVersion)
		if versionComparison == 1 {
			userProvidedAvagoVersion = constants.FujiAvalancheGoV113
		}
	}
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
		_, _, networkRPCVersion, err := localnet.GetLocalNetworkAvalancheGoVersion(app)
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

		if !sidecar.Sovereign {
			// sovereign blockchains are deployed into new local clusters,
			// non sovereign blockchains are deployed into the local network itself
			if b, err := localnet.BlockchainAlreadyDeployedOnLocalNetwork(app, blockchainName); err != nil {
				return err
			} else if b {
				return fmt.Errorf("blockchain %s has already been deployed", blockchainName)
			}
		}
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

	// TODO: will estimate fee in subsecuent PR
	// !subnetonly: add blockchain fee
	// createSubnet: add subnet fee
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
	// whether user has created Avalanche Nodes when blockchain deploy command is called
	if sidecar.Sovereign {
		if changeOwnerAddress == "" {
			// use provided key as change owner unless already set
			if pAddr, err := kc.PChainFormattedStrAddresses(); err == nil && len(pAddr) > 0 {
				changeOwnerAddress = pAddr[0]
				ux.Logger.PrintToUser("Using [%s] to be set as a change owner for leftover AVAX", changeOwnerAddress)
			}
		}
		if !generateNodeID {
			if cancel, err := StartLocalMachine(network, sidecar, blockchainName, deployBalance, availableBalance); err != nil {
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
	} else if network.Kind == models.Local {
		flags.NonSovFlags.SameControlKey = true
	}

	// from here on we are assuming a public deploy
	if subnetOnly && subnetIDStr != "" {
		return errMutuallyExlusiveSubnetFlags
	}

	if sidecar.Sovereign {
		requiredBalance := deployBalance * uint64(len(bootstrapValidators))
		if availableBalance < requiredBalance {
			return fmt.Errorf(
				"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
				len(bootstrapValidators),
				requiredBalance,
				availableBalance,
			)
		}
	}

	network.HandlePublicNetworkSimulation()

	if createSubnet {
		if sidecar.Sovereign {
			flags.NonSovFlags.SameControlKey = true
		}
		err = promptOwners(
			kc,
			&flags.NonSovFlags,
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
		isPermissioned, flags.NonSovFlags.ControlKeys, flags.NonSovFlags.Threshold, err = txutils.GetOwners(network, subnetID)
		if err != nil {
			return err
		}
		if !isPermissioned {
			return ErrNotPermissionedSubnet
		}
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(flags.NonSovFlags.ControlKeys); err != nil {
		return err
	}

	kcKeys, err := kc.PChainFormattedStrAddresses()
	if err != nil {
		return err
	}

	// get keys for blockchain tx signing
	if flags.NonSovFlags.SubnetAuthKeys != nil {
		if err := prompts.CheckSubnetAuthKeys(kcKeys, flags.NonSovFlags); err != nil {
			return err
		}
	} else {
		err = prompts.SetSubnetAuthKeys(app.Prompt, kcKeys, &flags.NonSovFlags)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Your blockchain auth keys for chain creation: %s", flags.NonSovFlags.SubnetAuthKeys)

	// deploy to public network
	deployer := subnet.NewPublicDeployer(app, kc, network)

	if createSubnet {
		subnetID, err = deployer.DeploySubnet(flags.NonSovFlags)
		if err != nil {
			return err
		}
		deployer.CleanCacheWallet()
		// get the control keys in the same order as the tx
		_, flags.NonSovFlags.ControlKeys, flags.NonSovFlags.Threshold, err = txutils.GetOwners(network, subnetID)
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
			flags.NonSovFlags,
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
			flags.NonSovFlags,
			remainingSubnetAuthKeys,
			false,
		); err != nil {
			return err
		}
	}

	tracked := false

	if sidecar.Sovereign {
		validatorManagerStr := validatorManagerSDK.ProxyContractAddress
		avaGoBootstrapValidators, cancel, savePartialTx, err := convertSubnetToL1(
			bootstrapValidators,
			deployer,
			subnetID,
			blockchainID,
			network,
			chain,
			sidecar,
			flags.NonSovFlags,
			validatorManagerStr,
			false,
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
		if convertOnly || generateNodeID || (!useLocalMachine && clusterNameFlagValue == "") {
			ux.Logger.GreenCheckmarkToUser("Converted blockchain successfully generated")
			ux.Logger.PrintToUser("To finish conversion to sovereign L1, create the corresponding Avalanche node(s) with the provided Node ID and BLS Info")
			ux.Logger.PrintToUser("Created Node ID and BLS Info can be found at %s", app.GetSidecarPath(blockchainName))
			ux.Logger.PrintToUser("==================================================")
			ux.Logger.PrintToUser("To enable the nodes to track the L1, set '%s' as the value for 'track-subnets' configuration in ~/.avalanchego/config.json", subnetID)
			ux.Logger.PrintToUser("Ensure that the P2P port is exposed and 'public-ip' config value is set")
			ux.Logger.PrintToUser("Once the Avalanche Node(s) are created and are tracking the blockchain, call `avalanche contract initValidatorManager %s` to finish conversion to sovereign L1", blockchainName)
			return nil
		}

		tracked, err = InitializeValidatorManager(
			blockchainName,
			sidecar.ValidatorManagerOwner,
			subnetID,
			blockchainID,
			network,
			avaGoBootstrapValidators,
			sidecar.ValidatorManagement == models.ProofOfStake,
			validatorManagerStr,
			sidecar.ProxyContractOwner,
			sidecar.UseACP99,
			flags.SigAggFlags,
		)
		if err != nil {
			return err
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
			"",
		); err != nil {
			return err
		}
		if network.Kind == models.Local && !simulatedPublicNetwork() {
			ux.Logger.PrintToUser("")
			ctx, cancel := localnet.GetLocalNetworkDefaultContext()
			defer cancel()
			if err := localnet.LocalNetworkTrackSubnet(
				ctx,
				app,
				blockchainName,
			); err != nil {
				return err
			}
			tracked = true
		}
	}

	if sidecar.Sovereign && tracked {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Your L1 is ready for on-chain interactions."))
	}

	var icmErr, relayerErr error
	if sidecar.TeleporterReady && tracked && !icmSpec.SkipICMDeploy {
		chainSpec := contract.ChainSpec{
			BlockchainName: blockchainName,
		}
		chainSpec.SetEnabled(true, false, false, false, false)
		deployICMFlags := messengercmd.DeployFlags{
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
		if err := messengercmd.CallDeploy([]string{}, deployICMFlags, network); err != nil {
			icmErr = err
			ux.Logger.RedXToUser("Interchain Messaging is not deployed due to: %v", icmErr)
		} else {
			ux.Logger.GreenCheckmarkToUser("ICM is successfully deployed")
			if network.Kind != models.Local && !useLocalMachine {
				if flag := cmd.Flags().Lookup(skipRelayerFlagName); flag != nil && !flag.Changed {
					ux.Logger.PrintToUser("")
					yes, err := app.Prompt.CaptureYesNo("Do you want to setup local relayer for the messages to be interchanged, as Interchain Messaging was deployed to your blockchain?")
					if err != nil {
						return err
					}
					icmSpec.SkipRelayerDeploy = !yes
				}
			}
			if !icmSpec.SkipRelayerDeploy && network.Kind != models.Mainnet {
				deployRelayerFlags := relayercmd.DeployFlags{
					Version:            icmSpec.RelayerVersion,
					BinPath:            icmSpec.RelayerBinPath,
					LogLevel:           icmSpec.RelayerLogLevel,
					RelayCChain:        relayCChain,
					CChainFundingKey:   cChainFundingKey,
					BlockchainsToRelay: []string{blockchainName},
					Key:                relayerKeyName,
					Amount:             relayerAmount,
					AllowPrivateIPs:    relayerAllowPrivateIPs,
				}
				if network.Kind == models.Local || useLocalMachine {
					deployRelayerFlags.Key = constants.ICMRelayerKeyName
					deployRelayerFlags.Amount = constants.DefaultRelayerAmount
					deployRelayerFlags.BlockchainFundingKey = constants.ICMKeyName
				}
				if network.Kind == models.Local {
					deployRelayerFlags.CChainFundingKey = "ewoq"
					deployRelayerFlags.CChainAmount = constants.DefaultRelayerAmount
				}
				if err := relayercmd.CallDeploy(nil, deployRelayerFlags, network); err != nil {
					relayerErr = err
					ux.Logger.RedXToUser("Relayer is not deployed due to: %v", relayerErr)
				} else {
					ux.Logger.GreenCheckmarkToUser("Relayer is successfully deployed")
				}
			}
		}
	}

	flags := make(map[string]string)
	flags[constants.MetricsNetwork] = network.Name()
	metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)

	if network.Kind == models.Local && !simulatedPublicNetwork() {
		ux.Logger.PrintToUser("")
		_ = PrintSubnetInfo(blockchainName, true)
	}
	if icmErr != nil {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Interchain Messaging is not deployed due to: %v", icmErr)
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To deploy ICM later on, call `avalanche icm deploy`")
		ux.Logger.PrintToUser("This does not affect L1 operations besides Interchain Messaging")
	}
	if relayerErr != nil {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Relayer is not deployed due to: %v", relayerErr)
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To deploy a local relayer later on, call `avalanche interchain relayer deploy`")
		ux.Logger.PrintToUser("This does not affect L1 operations besides Interchain Messaging")
	}

	if tracked {
		if sidecar.Sovereign {
			ux.Logger.GreenCheckmarkToUser("L1 is successfully deployed on %s", network.Name())
		} else {
			ux.Logger.GreenCheckmarkToUser("Subnet is successfully deployed on %s", network.Name())
		}
	}

	return nil
}

func setBootstrapValidatorValidationID(avaGoBootstrapValidators []*txs.ConvertSubnetToL1Validator, bootstrapValidators []models.SubnetValidator, subnetID ids.ID) {
	for index, avagoValidator := range avaGoBootstrapValidators {
		for bootstrapValidatorIndex, validator := range bootstrapValidators {
			avagoValidatorNodeID, _ := ids.ToNodeID(avagoValidator.NodeID)
			if validator.NodeID == avagoValidatorNodeID.String() {
				validationID := subnetID.Append(uint32(index))
				bootstrapValidators[bootstrapValidatorIndex].ValidationID = validationID.String()
			}
		}
	}
}

func getClusterBootstrapValidators(
	clusterName string,
	network models.Network,
	deployBalance uint64,
) ([]models.SubnetValidator, error) {
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
		changeAddr, err = blockchain.GetKeyForChangeOwner(app, network)
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
			Balance:              deployBalance,
			BLSPublicKey:         fmt.Sprintf("%s%s", "0x", hex.EncodeToString(pub)),
			BLSProofOfPossession: fmt.Sprintf("%s%s", "0x", hex.EncodeToString(pop)),
			ChangeOwnerAddr:      changeAddr,
		})
	}
	return subnetValidators, nil
}

// TODO: add deactivation owner?
func ConvertToAvalancheGoSubnetValidator(subnetValidators []models.SubnetValidator) ([]*txs.ConvertSubnetToL1Validator, error) {
	bootstrapValidators := []*txs.ConvertSubnetToL1Validator{}
	for _, validator := range subnetValidators {
		nodeID, err := ids.NodeIDFromString(validator.NodeID)
		if err != nil {
			return nil, err
		}
		blsInfo, err := blockchain.ConvertToBLSProofOfPossession(validator.BLSPublicKey, validator.BLSProofOfPossession)
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
		return nil, fmt.Errorf("blockchain name %s is invalid: %w", args[0], err)
	}
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return nil, errors.New("Invalid blockchain " + args[0])
	}

	return chains, nil
}

func SaveNotFullySignedTx(
	txName string,
	tx *txs.Tx,
	blockchainName string,
	subnetFlags flags.SubnetFlags,
	remainingSubnetAuthKeys []string,
	forceOverwrite bool,
) error {
	signedCount := len(subnetFlags.SubnetAuthKeys) - len(remainingSubnetAuthKeys)
	ux.Logger.PrintToUser("")
	if signedCount == len(subnetFlags.SubnetAuthKeys) {
		ux.Logger.PrintToUser("All %d required %s signatures have been signed. "+
			"Saving tx to disk to enable commit.", len(subnetFlags.SubnetAuthKeys), txName)
	} else {
		ux.Logger.PrintToUser("%d of %d required %s signatures have been signed. "+
			"Saving tx to disk to enable remaining signing.", signedCount, len(subnetFlags.SubnetAuthKeys), txName)
	}
	if subnetFlags.OutputTxPath == "" {
		ux.Logger.PrintToUser("")
		var err error
		if forceOverwrite {
			subnetFlags.OutputTxPath, err = app.Prompt.CaptureString("Path to export partially signed tx to")
		} else {
			subnetFlags.OutputTxPath, err = app.Prompt.CaptureNewFilepath("Path to export partially signed tx to")
		}
		if err != nil {
			return err
		}
	}
	if forceOverwrite {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Overwriting %s", subnetFlags.OutputTxPath)
	}
	if err := txutils.SaveToDisk(tx, subnetFlags.OutputTxPath, forceOverwrite); err != nil {
		return err
	}
	if signedCount == len(subnetFlags.SubnetAuthKeys) {
		PrintReadyToSignMsg(blockchainName, subnetFlags.OutputTxPath)
	} else {
		PrintRemainingToSignMsg(blockchainName, remainingSubnetAuthKeys, subnetFlags.OutputTxPath)
	}
	return nil
}

func PrintReadyToSignMsg(
	blockchainName string,
	outputTxPath string,
) {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Tx is fully signed, and ready to be committed")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Commit command:")
	cmdLine := fmt.Sprintf("  avalanche transaction commit %s --input-tx-filepath %s", blockchainName, outputTxPath)
	if blockchainName == "" {
		cmdLine = fmt.Sprintf("  avalanche transaction commit --input-tx-filepath %s", outputTxPath)
	}
	ux.Logger.PrintToUser(cmdLine)
}

func PrintRemainingToSignMsg(
	blockchainName string,
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
	cmdline := fmt.Sprintf("  avalanche transaction sign %s --input-tx-filepath %s", blockchainName, outputTxPath)
	if blockchainName == "" {
		cmdline = fmt.Sprintf("  avalanche transaction sign --input-tx-filepath %s", outputTxPath)
	}
	ux.Logger.PrintToUser(cmdline)
	ux.Logger.PrintToUser("")
}

func PrintDeployResults(blockchainName string, subnetID ids.ID, blockchainID ids.ID) error {
	t := ux.DefaultTable("Deployment results", nil)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, AutoMerge: true},
	})
	if blockchainName != "" {
		t.AppendRow(table.Row{"Chain Name", blockchainName})
	}
	t.AppendRow(table.Row{"Subnet ID", subnetID.String()})
	if blockchainName != "" {
		vmID, err := anrutils.VMID(blockchainName)
		if err != nil {
			return fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
		}
		t.AppendRow(table.Row{"VM ID", vmID.String()})
	}
	if blockchainID != ids.Empty {
		t.AppendRow(table.Row{"Blockchain ID", blockchainID.String()})
		t.AppendRow(table.Row{"P-Chain TXID", blockchainID.String()})
	}
	ux.Logger.PrintToUser(t.Render())
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

func ConvertURIToPeers(uris []string) ([]info.Peer, error) {
	aggregatorPeers, err := blockchain.UrisToPeers(uris)
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

func simulatedPublicNetwork() bool {
	return os.Getenv(constants.SimulatePublicNetwork) != ""
}

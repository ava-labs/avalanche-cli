// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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

var deploySupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Devnet, networkoptions.Fuji, networkoptions.Mainnet}

var (
	sameControlKey           bool
	keyName                  string
	threshold                uint32
	controlKeys              []string
	subnetAuthKeys           []string
	userProvidedAvagoVersion string
	outputTxPath             string
	useLedger                bool
	useEwoq                  bool
	ledgerAddresses          []string
	subnetIDStr              string
	mainnetChainID           uint32
	skipCreatePrompt         bool
	avagoBinaryPath          string
	subnetOnly               bool
	teleporterEsp            subnet.TeleporterEsp

	errMutuallyExlusiveControlKeys = errors.New("--control-keys and --same-control-key are mutually exclusive")
	ErrMutuallyExlusiveKeyLedger   = errors.New("key source flags --key, --ledger/--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet          = errors.New("key --key is not available for mainnet operations")
	errMutuallyExlusiveSubnetFlags = errors.New("--subnet-only and --subnet-id are mutually exclusive")
)

// avalanche subnet deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys a subnet configuration",
		Long: `The subnet deploy command deploys your Subnet configuration locally, to Fuji Testnet, or to Mainnet.

At the end of the call, the command prints the RPC URL you can use to interact with the Subnet.

Avalanche-CLI only supports deploying an individual Subnet once per network. Subsequent
attempts to deploy the same Subnet to the same network (local, Fuji, Mainnet) aren't
allowed. If you'd like to redeploy a Subnet locally for testing, you must first call
avalanche network clean to reset all deployed chain state. Subsequent local deploys
redeploy the chain with fresh state. You can deploy the same Subnet to multiple networks,
so you can take your locally tested Subnet and deploy it on Fuji or Mainnet.`,
		RunE:              deploySubnet,
		PersistentPostRun: handlePostRun,
		Args:              cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, deploySupportedNetworkOptions)
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
	cmd.Flags().BoolVar(&teleporterEsp.SkipDeploy, "skip-local-teleporter", false, "skip automatic teleporter deploy on local networks [to be deprecated]")
	cmd.Flags().BoolVar(&teleporterEsp.SkipDeploy, "skip-teleporter-deploy", false, "skip automatic teleporter deploy")
	cmd.Flags().StringVar(&teleporterEsp.Version, "teleporter-version", "latest", "teleporter version to deploy")
	cmd.Flags().StringVar(&teleporterEsp.MessengerContractAddressPath, "teleporter-messenger-contract-address-path", "", "path to a teleporter messenger contract address file")
	cmd.Flags().StringVar(&teleporterEsp.MessengerDeployerAddressPath, "teleporter-messenger-deployer-address-path", "", "path to a teleporter messenger deployer address file")
	cmd.Flags().StringVar(&teleporterEsp.MessengerDeployerTxPath, "teleporter-messenger-deployer-tx-path", "", "path to a teleporter messenger deployer tx file")
	cmd.Flags().StringVar(&teleporterEsp.RegistryBydecodePath, "teleporter-registry-bytecode-path", "", "path to a teleporter registry bytecode file")
	return cmd
}

func CallDeploy(
	cmd *cobra.Command,
	subnetOnlyParam bool,
	subnetName string,
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
	return deploySubnet(cmd, []string{subnetName})
}

func getChainsInSubnet(subnetName string) ([]string, error) {
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
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

func checkSubnetEVMDefaultAddressNotInAlloc(network models.Network, chain string) error {
	if network.Kind != models.Local && network.Kind != models.Devnet && os.Getenv(constants.SimulatePublicNetwork) == "" {
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
	return deploySubnet(cmd, args)
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
func getSubnetEVMMainnetChainID(sc *models.Sidecar, subnetName string) error {
	// get original chain id
	evmGenesis, err := app.LoadEvmGenesis(subnetName)
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

// deploySubnet is the cobra command run for deploying subnets
func deploySubnet(cmd *cobra.Command, args []string) error {
	subnetName := args[0]

	if err := CreateSubnetFirst(cmd, subnetName, skipCreatePrompt); err != nil {
		return err
	}

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	if teleporterEsp.MessengerContractAddressPath != "" || teleporterEsp.MessengerDeployerAddressPath != "" || teleporterEsp.MessengerDeployerTxPath != "" || teleporterEsp.RegistryBydecodePath != "" {
		if teleporterEsp.MessengerContractAddressPath == "" || teleporterEsp.MessengerDeployerAddressPath == "" || teleporterEsp.MessengerDeployerTxPath == "" || teleporterEsp.RegistryBydecodePath == "" {
			return fmt.Errorf("if setting any teleporter asset path, you must set all teleporter asset paths")
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
		deployInfo, err := deployer.DeployToLocalNetwork(chain, genesisPath, teleporterEsp, subnetIDStr)
		if err != nil {
			if deployer.BackendStartedHere() {
				if innerErr := binutils.KillgRPCServerProcess(app); innerErr != nil {
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
			ids.Empty,
			deployInfo.BlockchainID,
			deployInfo.TeleporterMessengerAddress,
			deployInfo.TeleporterRegistryAddress,
		); err != nil {
			return err
		}
		return PrintSubnetInfo(subnetName, true)
	}

	// from here on we are assuming a public deploy
	if subnetOnly && subnetIDStr != "" {
		return errMutuallyExlusiveSubnetFlags
	}

	createSubnet := true
	var subnetID, transferSubnetOwnershipTxID ids.ID
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
				transferSubnetOwnershipTxID = model.TransferSubnetOwnershipTxID
				createSubnet = false
			}
		}
	}

	fee := uint64(0)
	if !subnetOnly {
		fee += network.GenesisParams().CreateBlockchainTxFee
	}
	if createSubnet {
		fee += network.GenesisParams().CreateSubnetTxFee
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
			transferSubnetOwnershipTxID,
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

	if isFullySigned {
		if network.ClusterName != "" {
			clusterConfig, err := app.GetClusterConfig(network.ClusterName)
			if err != nil {
				return err
			}
			if _, err := utils.GetIndexInSlice(clusterConfig.Subnets, subnetName); err != nil {
				clusterConfig.Subnets = append(clusterConfig.Subnets, subnetName)
			}
			if err := app.SetClusterConfig(network.ClusterName, clusterConfig); err != nil {
				return err
			}
		}
	}

	flags := make(map[string]string)
	flags[constants.MetricsNetwork] = network.Name()
	metrics.HandleTracking(cmd, constants.MetricsSubnetDeployCommand, app, flags)

	// update sidecar
	// TODO: need to do something for backwards compatibility?
	return app.UpdateSidecarNetworks(&sidecar, network, subnetID, transferSubnetOwnershipTxID, blockchainID, "", "")
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

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-network-runner/server"

	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/formatting/address"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/plugins"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/spf13/cobra"
)

const ewoqPChainAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"

var (
	// path to avalanchego config file
	avagoConfigPath string
	// path to avalanchego plugin dir
	pluginDir string
	// path to avalanchego datadir dir
	dataDir string
	// if true, print the manual instructions to screen
	printManual bool
	// if true, doesn't ask for overwriting the config file
	forceWrite bool
	// if true, validator is joining a permissionless subnet
	joinElastic bool
	// for permissionless subnet only: how much subnet native token will be staked in the validator
	stakeAmount uint64

	errNoBlockchainID = errors.New("failed to find the blockchain ID for this subnet, has it been deployed/created on this network?")
)

// avalanche subnet deploy
func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [subnetName]",
		Short: "Configure your validator node to begin validating a new subnet",
		Long: `The subnet join command configures your validator node to begin validating a new Subnet.

To complete this process, you must have access to the machine running your validator. If the
CLI is running on the same machine as your validator, it can generate or update your node's
config file automatically. Alternatively, the command can print the necessary instructions
to update your node manually. To complete the validation process, the Subnet's admins must add
the NodeID of your validator to the Subnet's allow list by calling addValidator with your
NodeID.

After you update your validator's config, you need to restart your validator manually. If
you provide the --avalanchego-config flag, this command attempts to edit the config file
at that path.

This command currently only supports Subnets deployed on the Fuji Testnet and Mainnet.`,
		RunE: joinCmd,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&avagoConfigPath, "avalanchego-config", "", "file path of the avalanchego config file")
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "file path of avalanchego's plugin directory")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "path of avalanchego's data dir directory")
	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "join on `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "join on `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployLocal, "local", false, "join on `local` (for elastic subnet only)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "join on `mainnet`")
	cmd.Flags().BoolVar(&printManual, "print", false, "if true, print the manual config without prompting")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to check")
	cmd.Flags().BoolVar(&forceWrite, "force-write", false, "if true, skip to prompt to overwrite the config file")
	cmd.Flags().BoolVar(&joinElastic, "elastic", false, "set flag as true if joining elastic subnet")
	cmd.Flags().Uint64Var(&stakeAmount, "stake-amount", 0, "amount of tokens to stake on validator")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "start time that validator starts validating")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func joinCmd(_ *cobra.Command, args []string) error {
	if printManual && (avagoConfigPath != "" || pluginDir != "") {
		return errors.New("--print cannot be used with --avalanchego-config or --plugin-dir")
	}

	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	subnetName := chains[0]

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if !flags.EnsureMutuallyExclusive([]bool{deployMainnet, deployTestnet}) {
		return errors.New("--fuji and --mainnet are mutually exclusive")
	}

	var network models.Network
	switch {
	case deployLocal:
		network = models.Local
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		if joinElastic {
			selectedNetwork, err := promptNetworkElastic(sc, "Which network is the elastic subnet that the node wants to join on?")
			if err != nil {
				return err
			}
			switch selectedNetwork {
			case localDeployment:
				network = models.Local
			case fujiDeployment:
				network = models.Fuji
			case mainnetDeployment:
				return errors.New("joining elastic subnet is not yet supported on Mainnet")
			}
		} else {
			networkStr, err := app.Prompt.CaptureList(
				"Choose a network to validate on (this command only supports public networks)",
				[]string{models.Fuji.String(), models.Mainnet.String()},
			)
			if err != nil {
				return err
			}
			// flag provided
			networkStr = strings.Title(networkStr)
			// as we are allowing a flag, we need to check if a supported network has been provided
			if !(networkStr == models.Fuji.String() || networkStr == models.Mainnet.String()) {
				return errors.New("unsupported network")
			}
			network = models.NetworkFromString(networkStr)
		}
	}

	if joinElastic {
		return handleValidatorJoinElasticSubnet(sc, network, subnetName)
	}

	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
	}

	networkLower := strings.ToLower(network.String())

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	subnetIDStr := subnetID.String()

	if printManual {
		pluginDir = app.GetTmpPluginDir()
		vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
		if err != nil {
			return err
		}
		printJoinCmd(subnetIDStr, networkLower, vmPath)
		return nil
	}

	// if **both** flags were set, nothing special needs to be done
	// just check the following blocks
	if avagoConfigPath == "" && pluginDir == "" {
		// both flags are NOT set
		const (
			choiceManual    = "Manual"
			choiceAutomatic = "Automatic"
		)
		choice, err := app.Prompt.CaptureList(
			"How would you like to update the avalanchego config?",
			[]string{choiceAutomatic, choiceManual},
		)
		if err != nil {
			return err
		}
		if choice == choiceManual {
			pluginDir = app.GetTmpPluginDir()
			vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
			if err != nil {
				return err
			}
			printJoinCmd(subnetIDStr, networkLower, vmPath)
			return nil
		}
	}

	// if choice is automatic, we just pass through this block
	// or, pluginDir was set but not avagoConfigPath
	// if **both** flags were set, this will be skipped...
	if avagoConfigPath == "" {
		avagoConfigPath, err = plugins.FindAvagoConfigPath()
		if err != nil {
			return err
		}
		if avagoConfigPath != "" {
			ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Green.Wrap("Found a config file at %s")), avagoConfigPath)
			yes, err := app.Prompt.CaptureYesNo("Is this the file we should update?")
			if err != nil {
				return err
			}
			if yes {
				ux.Logger.PrintToUser("Will use file at path %s to update the configuration", avagoConfigPath)
			} else {
				avagoConfigPath = ""
			}
		}
		if avagoConfigPath == "" {
			avagoConfigPath, err = app.Prompt.CaptureString("Path to your existing config file (or where it will be generated)")
			if err != nil {
				return err
			}
		}
	}

	// ...but not this
	avagoConfigPath, err := plugins.SanitizePath(avagoConfigPath)
	if err != nil {
		return err
	}

	// avagoConfigPath was set but not pluginDir
	// if **both** flags were set, this will be skipped...
	if pluginDir == "" {
		pluginDir, err = plugins.FindPluginDir()
		if err != nil {
			return err
		}
		if pluginDir != "" {
			ux.Logger.PrintToUser(logging.Bold.Wrap(logging.Green.Wrap("Found the VM plugin directory at %s")), pluginDir)
			yes, err := app.Prompt.CaptureYesNo("Is this where we should install the VM?")
			if err != nil {
				return err
			}
			if yes {
				ux.Logger.PrintToUser("Will use plugin directory at %s to install the VM", pluginDir)
			} else {
				pluginDir = ""
			}
		}
		if pluginDir == "" {
			pluginDir, err = app.Prompt.CaptureString("Path to your avalanchego plugin dir (likely avalanchego/build/plugins)")
			if err != nil {
				return err
			}
		}
	}

	// ...but not this
	pluginDir, err := plugins.SanitizePath(pluginDir)
	if err != nil {
		return err
	}

	vmPath, err := plugins.CreatePlugin(app, sc.Name, pluginDir)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("VM binary written to %s", vmPath)

	if forceWrite {
		if err := writeAvagoChainConfigFiles(app, dataDir, subnetName, sc, network); err != nil {
			return err
		}
	}

	subnetAvagoConfigFile := ""
	if app.AvagoNodeConfigExists(subnetName) {
		subnetAvagoConfigFile = app.GetAvagoNodeConfigPath(subnetName)
	}

	if err := plugins.EditConfigFile(
		app,
		subnetIDStr,
		networkLower,
		avagoConfigPath,
		forceWrite,
		subnetAvagoConfigFile,
	); err != nil {
		return err
	}

	return nil
}

func writeAvagoChainConfigFiles(
	app *application.Avalanche,
	dataDir string,
	subnetName string,
	sc models.Sidecar,
	network models.Network,
) error {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataDir = filepath.Join(home, ".avalanchego")
	}

	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	subnetIDStr := subnetID.String()
	blockchainID := sc.Networks[network.String()].BlockchainID
	if blockchainID == ids.Empty {
		return errNoBlockchainID
	}
	blockchainIDStr := blockchainID.String()

	configsPath := filepath.Join(dataDir, "configs")

	subnetConfigsPath := filepath.Join(configsPath, "subnets")
	subnetConfigPath := filepath.Join(subnetConfigsPath, subnetIDStr+".json")
	if app.AvagoSubnetConfigExists(subnetName) {
		if err := os.MkdirAll(subnetConfigsPath, constants.DefaultPerms755); err != nil {
			return err
		}
		subnetConfig, err := app.LoadRawAvagoSubnetConfig(subnetName)
		if err != nil {
			return err
		}
		if err := os.WriteFile(subnetConfigPath, subnetConfig, constants.DefaultPerms755); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(subnetConfigPath)
	}

	if app.ChainConfigExists(subnetName) || app.NetworkUpgradeExists(subnetName) {
		chainConfigsPath := filepath.Join(configsPath, "chains", blockchainIDStr)
		if err := os.MkdirAll(chainConfigsPath, constants.DefaultPerms755); err != nil {
			return err
		}
		chainConfigPath := filepath.Join(chainConfigsPath, "config.json")
		if app.ChainConfigExists(subnetName) {
			chainConfig, err := app.LoadRawChainConfig(subnetName)
			if err != nil {
				return err
			}
			if err := os.WriteFile(chainConfigPath, chainConfig, constants.DefaultPerms755); err != nil {
				return err
			}
		} else {
			_ = os.RemoveAll(chainConfigPath)
		}
		networkUpgradesPath := filepath.Join(chainConfigsPath, "upgrade.json")
		if app.NetworkUpgradeExists(subnetName) {
			networkUpgrades, err := app.LoadRawNetworkUpgrades(subnetName)
			if err != nil {
				return err
			}
			if err := os.WriteFile(networkUpgradesPath, networkUpgrades, constants.DefaultPerms755); err != nil {
				return err
			}
		} else {
			_ = os.RemoveAll(networkUpgradesPath)
		}
	}

	return nil
}

func handleValidatorJoinElasticSubnet(sc models.Sidecar, network models.Network, subnetName string) error {
	var err error
	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	subnetID := sc.Networks[network.String()].SubnetID
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		subnetID = sc.Networks[models.Local.String()].SubnetID
	}
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	nodeID, err := promptNodeIDToAdd(subnetID, true, network)
	if err != nil {
		return err
	}
	stakedTokenAmount, err := promptStakeAmount(subnetName, true, network)
	if err != nil {
		return err
	}
	start, stakeDuration, err := getTimeParameters(network, nodeID, true)
	if err != nil {
		return err
	}
	endTime := start.Add(stakeDuration)
	ux.Logger.PrintToUser("Inputs complete, issuing transaction for the provided validator to join elastic subnet...")
	ux.Logger.PrintToUser("")
	switch network {
	case models.Local:
		return handleValidatorJoinElasticSubnetLocal(sc, network, subnetName, nodeID, stakedTokenAmount, start, endTime)
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, "pay transaction fees", app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		return errors.New("unsupported network")
	default:
		return errors.New("unsupported network")
	}
	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.Local
	}

	// get keychain accessor
	kc, err := GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	recipientAddr := kc.Addresses().List()[0]
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	assetID, err := getSubnetAssetID(subnetID, network)
	if err != nil {
		return err
	}
	txID, err := deployer.AddPermissionlessValidator(subnetID, assetID, nodeID, stakedTokenAmount, uint64(start.Unix()), uint64(endTime.Unix()), recipientAddr)
	if err != nil {
		return err
	}
	printAddPermissionlessValOutput(txID, nodeID, network, start, endTime, stakedTokenAmount)
	if err = app.UpdateSidecarPermissionlessValidator(&sc, network, nodeID.String(), txID); err != nil {
		return fmt.Errorf("joining permissionless subnet was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

func getSubnetAssetID(subnetID ids.ID, network models.Network) (ids.ID, error) {
	var api string
	switch network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	case models.Local:
		api = constants.LocalAPIEndpoint
	default:
		return ids.Empty, fmt.Errorf("network not supported")
	}

	pClient := platformvm.NewClient(api)
	ctx := context.Background()
	assetID, err := pClient.GetStakingAssetID(ctx, subnetID)
	if err != nil {
		return ids.Empty, err
	}
	return assetID, nil
}

func printAddPermissionlessValOutput(txID ids.ID, nodeID ids.NodeID, network models.Network, start time.Time, endTime time.Time, stakedTokenAmount uint64) {
	ux.Logger.PrintToUser("Validator successfully joined elastic subnet!")
	ux.Logger.PrintToUser("TX ID: %s", txID.String())
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.UTC().Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", endTime.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("Stake Amount: %d", stakedTokenAmount)
}

func handleValidatorJoinElasticSubnetLocal(sc models.Sidecar, network models.Network, subnetName string, nodeID ids.NodeID,
	stakedTokenAmount uint64, start time.Time, endTime time.Time,
) error {
	if network != models.Local {
		return errors.New("unsupported network")
	}
	if !checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf("%s is not an elastic subnet", subnetName)
	}
	assetID := sc.ElasticSubnet[models.Local.String()].AssetID
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	subnetID := sc.Networks[models.Local.String()].SubnetID
	txID, err := subnet.IssueAddPermissionlessValidatorTx(keyChain, subnetID, nodeID, stakedTokenAmount, assetID, uint64(start.Unix()), uint64(endTime.Unix()))
	if err != nil {
		return err
	}
	printAddPermissionlessValOutput(txID, nodeID, network, start, endTime, stakedTokenAmount)
	if err = app.UpdateSidecarPermissionlessValidator(&sc, models.Local, nodeID.String(), txID); err != nil {
		return fmt.Errorf("joining permissionless subnet was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

func checkIsValidating(subnetID ids.ID, nodeID ids.NodeID, pClient platformvm.Client) (bool, error) {
	// first check if the node is already an accepted validator on the subnet
	ctx := context.Background()
	nodeIDs := []ids.NodeID{nodeID}
	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nodeIDs)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		// strictly this is not needed, as we are providing the nodeID as param
		// just a double check
		if v.NodeID == nodeID {
			return true, nil
		}
	}

	// if not, also check the pending validator set
	pVals, _, err := pClient.GetPendingValidators(ctx, subnetID, nodeIDs)
	if err != nil {
		return false, err
	}
	// pVals is an array of interfaces as it can be of different types
	// but it's content is a JSON map[string]interface{}
	for _, iv := range pVals {
		if v, ok := iv.(map[string]interface{}); ok {
			// strictly this is not needed, as we are providing the nodeID as param
			// just a double check
			if v["nodeID"] == nodeID.String() {
				return true, nil
			}
		}
	}

	return false, nil
}

func getLocalNetworkIDs() ([]string, error) {
	var localNodeIDs []string
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx := binutils.GetAsyncContext()
	status, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser("No local network running")
			return nil, nil
		}
		return nil, err
	}

	if status != nil && status.ClusterInfo != nil {
		for _, val := range status.ClusterInfo.NodeInfos {
			localNodeIDs = append(localNodeIDs, val.Id)
		}
	}
	return localNodeIDs, nil
}

func promptNodeIDToAdd(subnetID ids.ID, isValidator bool, network models.Network) (ids.NodeID, error) {
	if nodeIDStr == "" {
		if network != models.Local {
			promptStr := "Please enter the Node ID of the node that you would like to add to the elastic subnet"
			if !isValidator {
				promptStr = "Please enter the Node ID of the validator that you would like to delegate to"
			}
			ux.Logger.PrintToUser(promptStr)
			return app.Prompt.CaptureNodeID("Node ID (format it as NodeID-<node_id>)")
		}
		defaultLocalNetworkNodeIDs, err := getLocalNetworkIDs()
		if err != nil {
			return ids.EmptyNodeID, err
		}
		// Get NodeIDs of all validators on the subnet
		validators, err := subnet.GetSubnetValidators(subnetID)
		if err != nil {
			return ids.EmptyNodeID, err
		}
		// construct list of validators to choose from
		var validatorList []string
		valNodeIDsMap := make(map[string]bool)
		for _, val := range validators {
			valNodeIDsMap[val.NodeID.String()] = true
		}
		if !isValidator {
			for _, v := range validators {
				validatorList = append(validatorList, v.NodeID.String())
			}
		} else {
			for _, localNodeID := range defaultLocalNetworkNodeIDs {
				if _, ok := valNodeIDsMap[localNodeID]; !ok {
					validatorList = append(validatorList, localNodeID)
				}
			}
		}
		promptStr := "Which validator you'd like to join this elastic subnet?"
		if !isValidator {
			promptStr = "Which validator would you like to delegate to?"
		}
		nodeIDStr, err = app.Prompt.CaptureList(promptStr, validatorList)
		if err != nil {
			return ids.EmptyNodeID, err
		}
	}
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return ids.NodeID{}, err
	}
	return nodeID, nil
}

func promptStakeAmount(subnetName string, isValidator bool, network models.Network) (uint64, error) {
	if stakeAmount > 0 {
		return stakeAmount, nil
	}
	if network == models.Local {
		esc, err := app.LoadElasticSubnetConfig(subnetName)
		if err != nil {
			return 0, err
		}
		maxValidatorStake := fmt.Sprintf("Maximum Validator Stake (%d)", esc.MaxValidatorStake)
		customWeight := fmt.Sprintf("Custom (Has to be between minValidatorStake (%d) and maxValidatorStake (%d) defined during elastic subnet transformation)", esc.MinValidatorStake, esc.MaxValidatorStake)
		if !isValidator {
			customWeight = fmt.Sprintf("Custom (Has to be between minDelegatorStake (%d) and maxValidatorStake (%d) defined during elastic subnet transformation)", esc.MinDelegatorStake, esc.MaxValidatorStake)
		}

		txt := "What amount of the subnet native token would you like to stake?"
		weightOptions := []string{maxValidatorStake, customWeight}
		weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
		if err != nil {
			return 0, err
		}
		ctx := context.Background()
		pClient := platformvm.NewClient(constants.LocalAPIEndpoint)
		walletBalance, err := getAssetBalance(ctx, pClient, ewoqPChainAddr, esc.AssetID)
		if err != nil {
			return 0, err
		}
		minStakePromptStr := fmt.Sprintf("Min Validator Stake(%d)", esc.MinValidatorStake)
		minStakeVal := esc.MinValidatorStake
		if !isValidator {
			minStakePromptStr = fmt.Sprintf("Min Delegator Stake(%d)", esc.MinValidatorStake)
			minStakeVal = esc.MinDelegatorStake
		}
		switch weightOption {
		case maxValidatorStake:
			return esc.MaxValidatorStake, nil
		default:
			return app.Prompt.CaptureUint64Compare(
				txt,
				[]prompts.Comparator{
					{
						Label: fmt.Sprintf("Max Validator Stake(%d)", esc.MaxValidatorStake),
						Type:  prompts.LessThanEq,
						Value: esc.MaxValidatorStake,
					},
					{
						Label: minStakePromptStr,
						Type:  prompts.MoreThanEq,
						Value: minStakeVal,
					},
					{
						Label: fmt.Sprintf("Wallet Balance(%d)", walletBalance),
						Type:  prompts.LessThanEq,
						Value: walletBalance,
					},
				},
			)
		}
	}
	ux.Logger.PrintToUser("What amount of the subnet native token would you like to stake?")
	initialSupply, err := app.Prompt.CaptureUint64("Stake amount")
	if err != nil {
		return 0, err
	}
	return initialSupply, nil
}

func printJoinCmd(subnetID string, networkID string, vmPath string) {
	msg := `
To setup your node, you must do two things:

1. Add your VM binary to your node's plugin directory
2. Update your node config to start validating the subnet

To add the VM to your plugin directory, copy or scp from %s

If you installed avalanchego with the install script, your plugin directory is likely
~/.avalanchego/build/plugins.

If you start your node from the command line WITHOUT a config file (e.g. via command
line or systemd script), add the following flag to your node's startup command:

--track-subnets=%s
(if the node already has a track-subnets config, append the new value by
comma-separating it).

For example:
./build/avalanchego --network-id=%s --track-subnets=%s

If you start the node via a JSON config file, add this to your config file:
track-subnets: %s

NOTE: The flag --track-subnets is a replacement of the deprecated --whitelisted-subnets.
If the later is present in config, please rename it to track-subnets first.

TIP: Try this command with the --avalanchego-config flag pointing to your config file,
this tool will try to update the file automatically (make sure it can write to it).

After you update your config, you will need to restart your node for the changes to
take effect.`

	ux.Logger.PrintToUser(msg, vmPath, subnetID, networkID, subnetID, subnetID)
}

func getAssetBalance(ctx context.Context, pClient platformvm.Client, addr string, assetID ids.ID) (uint64, error) {
	pID, err := address.ParseToID(addr)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, constants.RequestTimeout)
	resp, err := pClient.GetBalance(ctx, []ids.ShortID{pID})
	cancel()
	if err != nil {
		return 0, err
	}
	assetIDBalance := resp.Balances[assetID]
	return uint64(assetIDBalance), nil
}

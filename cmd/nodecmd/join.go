// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanchego/vms/platformvm"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	deployTestnet   bool
	deployMainnet   bool
	keyName         string
	subnetName      string
	useLedger       bool
	ledgerAddresses []string
	weight          uint64
	duration        time.Duration

	ErrMutuallyExlusiveKeyLedger = errors.New("--key and --ledger,--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet        = errors.New("--key is not available for mainnet operations")
	ErrNoBlockchainID            = errors.New("failed to find the blockchain ID for this subnet, has it been deployed/created on this network?")
)

func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [clusterName]",
		Short: "(ALPHA Warning) Join a subnet as a validator",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node join command enables all nodes in a cluster to be validators of a Subnet.
The command also enables nodes to be Primary Network validators by omitting the --subnet flag.
If The command is run before the nodes are Primary Network validators, the command will first
make the nodes Primary Network validators before making them Subnet validators. 
If The command is run before the nodes are bootstrapped on the Primary Network, the command will fail. 
You can check the bootstrap status by calling avalanche node status <clusteName>`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         joinSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is validating")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "set up validator in testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "set up validator in fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "set up validator in mainnet")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().Uint64Var(&weight, "stake-amount", 0, "how many AVAXSymbol to stake in the validator")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")

	return cmd
}

func parseBootstrappedOutput(filePath string) (bool, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return false, err
	}
	isBootstrappedInterface, ok := result["result"].(map[string]interface{})
	if ok {
		isBootstrapped, ok := isBootstrappedInterface["isBootstrapped"].(bool)
		if ok {
			return isBootstrapped, nil
		}
	}
	return false, nil
}

func parseSubnetSyncOutput(filePath string) (string, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return "", err
	}
	statusInterface, ok := result["result"].(map[string]interface{})
	if ok {
		status, ok := statusInterface["status"].(string)
		if ok {
			return status, nil
		}
	}
	return "", nil
}

func parseNodeIDOutput(fileName string) (string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	if err = json.Unmarshal(byteValue, &result); err != nil {
		return "", err
	}
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		nodeID, ok := nodeIDInterface["nodeID"].(string)
		if ok {
			return nodeID, nil
		}
	}
	return "", nil
}

func addNodeAsSubnetValidator(nodeID string, network models.Network) error {
	ux.Logger.PrintToUser("Adding the node as a Subnet Validator...")
	err := subnetcmd.CallAddValidator(subnetName, nodeID, network)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node successfully added as Subnet validator!")
	return nil
}

func getMinStakingAmount(network models.Network) (uint64, error) {
	var apiURL string
	switch network {
	case models.Mainnet:
		apiURL = constants.MainnetAPIEndpoint
	case models.Fuji:
		apiURL = constants.FujiAPIEndpoint
	}
	pClient := platformvm.NewClient(apiURL)
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()
	minValStake, _, err := pClient.GetMinStake(ctx, ids.Empty)
	if err != nil {
		return 0, err
	}
	return minValStake, nil
}

func validatePrimaryNetwork(nodeID ids.NodeID, network models.Network) error {
	ux.Logger.PrintToUser("Adding node as a Primary Network Validator...")
	var (
		start time.Time
		err   error
	)
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}
	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	switch network {
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, "pay transaction fees", app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}
	minValStake, err := getMinStakingAmount(network)
	if err != nil {
		return err
	}
	if weight == 0 {
		weight, err = promptWeightPrimaryNetwork(network)
		if err != nil {
			return err
		}
	}
	if weight < minValStake {
		return fmt.Errorf("illegal weight, must be greater than or equal to %d: %d", minValStake, weight)
	}
	start, duration, err = getTimeParametersPrimaryNetwork(network)
	if err != nil {
		return err
	}

	kc, err := subnetcmd.GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	recipientAddr := kc.Addresses().List()[0]
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	printNodeJoinOutput(nodeID, network, start)
	// we set the starting time for node to be a Primary Network Validator to be in 1 minute
	// we use min delegation fee as default
	// TODO: add prompt for delegation fee for mainnet
	delegationFee := genesis.FujiParams.MinDelegationFee
	if network == models.Mainnet {
		delegationFee = genesis.MainnetParams.MinDelegationFee
	}
	return deployer.AddValidatorPrimaryNetwork(nodeID, weight, start, duration, recipientAddr, delegationFee)
}

func promptWeightPrimaryNetwork(network models.Network) (uint64, error) {
	defaultStake := genesis.FujiParams.MinValidatorStake
	if network == models.Mainnet {
		defaultStake = genesis.MainnetParams.MinValidatorStake
	}
	defaultWeight := fmt.Sprintf("Default (%s)", convertToAVAXStr(defaultStake))
	txt := "What stake weight would you like to assign to the validator?"
	weightOptions := []string{defaultWeight, "Custom"}
	weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
	if err != nil {
		return 0, err
	}

	switch weightOption {
	case defaultWeight:
		return defaultStake, nil
	default:
		return app.Prompt.CaptureWeight(txt)
	}
}

func getTimeParametersPrimaryNetwork(network models.Network) (time.Time, time.Duration, error) {
	const (
		defaultDurationOption = "Minimum staking duration on primary network"
		custom                = "Custom"
	)
	start := time.Now().Add(constants.StakingStartLeadTime)
	if duration == 0 {
		msg := "How long should your validator validate for?"
		durationOptions := []string{defaultDurationOption, custom}
		durationOption, err := app.Prompt.CaptureList(msg, durationOptions)
		if err != nil {
			return time.Time{}, 0, err
		}

		switch durationOption {
		case defaultDurationOption:
			duration, err = getDefaultMaxValidationTime(start, network)
			if err != nil {
				return time.Time{}, 0, err
			}
		default:
			duration, err = subnetcmd.PromptDuration(start, network)
			if err != nil {
				return time.Time{}, 0, err
			}
		}
	}
	return start, duration, nil
}

func getDefaultMaxValidationTime(start time.Time, network models.Network) (time.Duration, error) {
	durationStr := constants.DefaultFujiStakeDuration
	if network == models.Mainnet {
		durationStr = constants.DefaultMainnetStakeDuration
	}
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}
	end := start.Add(d)
	confirm := fmt.Sprintf("Your validator will finish staking by %s", end.Format(constants.TimeParseLayout))
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return 0, err
	}
	if !yes {
		return 0, errors.New("you have to confirm staking duration")
	}
	return d, nil
}

func checkNodeIsBootstrapped(clusterName string) (bool, error) {
	ux.Logger.PrintToUser("Checking if node is bootstrapped to Primary Network ...")
	err := app.CreateFile(app.GetBootstrappedJSONFile())
	if err != nil {
		return false, err
	}
	if err := ansible.RunAnsiblePlaybookCheckBootstrapped(app.GetAnsibleDir(), app.GetBootstrappedJSONFile(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return false, err
	}
	isBootstrapped, err := parseBootstrappedOutput(app.GetBootstrappedJSONFile())
	if err != nil {
		return false, err
	}
	if err := app.RemoveFile(app.GetBootstrappedJSONFile()); err != nil {
		return false, err
	}
	if isBootstrapped {
		return true, nil
	}
	return false, nil
}

func getClusterNodeID(clusterName string) (string, error) {
	ux.Logger.PrintToUser("Getting node id ...")
	if err := app.CreateFile(app.GetNodeIDJSONFile()); err != nil {
		return "", err
	}
	if err := ansible.RunAnsiblePlaybookGetNodeID(app.GetAnsibleDir(), app.GetNodeIDJSONFile(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return "", err
	}
	nodeID, err := parseNodeIDOutput(app.GetNodeIDJSONFile())
	if err != nil {
		return "", err
	}
	if err = app.RemoveFile(app.GetNodeIDJSONFile()); err != nil {
		return "", err
	}
	return nodeID, err
}

func getClusterSubnetSyncStatus(blockchainID, clusterName string) (bool, error) {
	ux.Logger.PrintToUser("Checking if node is synced to subnet ...")
	if err := app.CreateFile(app.GetSubnetSyncJSONFile()); err != nil {
		return false, err
	}
	if err := ansible.RunAnsiblePlaybookSubnetSyncStatus(app.GetAnsibleDir(), app.GetSubnetSyncJSONFile(), blockchainID, app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return false, err
	}
	subnetSyncStatus, err := parseSubnetSyncOutput(app.GetSubnetSyncJSONFile())
	if err != nil {
		return false, err
	}
	if err = app.RemoveFile(app.GetSubnetSyncJSONFile()); err != nil {
		return false, err
	}
	if subnetSyncStatus == status.Syncing.String() {
		return true, nil
	} else if subnetSyncStatus == status.Validating.String() {
		return false, errors.New("node is already a subnet validator")
	}
	return false, nil
}

// checkNodeIsPrimaryNetworkValidator only returns err if node is already a Primary Network validator
func checkNodeIsPrimaryNetworkValidator(nodeID ids.NodeID, network models.Network) error {
	isValidator, err := subnet.IsSubnetValidator(ids.Empty, nodeID, network)
	if err != nil {
		ux.Logger.PrintToUser("failed to check if node is a validator on Primary Network: %s", err)
	} else if isValidator {
		return fmt.Errorf("node %s is already a validator on Primary Network", nodeID)
	}
	return nil
}

func addNodeAsPrimaryNetworkValidator(nodeID ids.NodeID, network models.Network) error {
	if err := checkNodeIsPrimaryNetworkValidator(nodeID, network); err != nil {
		err = validatePrimaryNetwork(nodeID, network)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Node successfully added as Primary Network validator!")
	}
	return nil
}

func waitForNodeToBePrimaryNetworkValidator(nodeID ids.NodeID) {
	ux.Logger.PrintToUser("Waiting 10 seconds for the node to be a Primary Network Validator...")
	// wait for 10 seconds because we set the start time to in 10 seconds
	time.Sleep(10 * time.Second)
	// long polling: try up to 5 times
	for i := 0; i < 5; i++ {
		// checkNodeIsPrimaryNetworkValidator only returns err if node is already a Primary Network validator
		if err := checkNodeIsPrimaryNetworkValidator(nodeID, models.Fuji); err != nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func joinSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	err := setupAnsible()
	if err != nil {
		return err
	}
	isBootstrapped, err := checkNodeIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	nodeIDStr, err := getClusterNodeID(clusterName)
	if err != nil {
		return err
	}
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	if subnetName == "" {
		// if no subnet is given in the flag, node will only be added as Primary Network Validator
		return addNodeAsPrimaryNetworkValidator(nodeID, models.Fuji)
	}
	if _, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	blockchainID := sc.Networks[models.Fuji.String()].BlockchainID
	if blockchainID == ids.Empty {
		return ErrNoBlockchainID
	}
	// we have to check if node is synced to subnet before adding the node as a validator
	isSubnetSynced, err := getClusterSubnetSyncStatus(blockchainID.String(), clusterName)
	if err != nil {
		return err
	}
	if !isSubnetSynced {
		return errors.New("node is not synced to subnet yet, please try again later")
	}
	if err = addNodeAsPrimaryNetworkValidator(nodeID, models.Fuji); err != nil {
		return err
	}
	waitForNodeToBePrimaryNetworkValidator(nodeID)
	return addNodeAsSubnetValidator(nodeIDStr, models.Fuji)
}

// convertToAVAXStr converts nanoAVAX to AVAXSymbol
func convertToAVAXStr(weight uint64) string {
	return fmt.Sprintf("%.9f %s", float64(weight)/float64(units.Avax), constants.AVAXSymbol)
}

func printNodeJoinOutput(nodeID ids.NodeID, network models.Network, start time.Time) {
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(duration).Format(constants.TimeParseLayout))
	// we need to divide by 10 ^ 9 since we were using nanoAvax
	ux.Logger.PrintToUser("Weight: %s", convertToAVAXStr(weight))
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")
}

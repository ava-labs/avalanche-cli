// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
		Use:   "join [subnetName]",
		Short: "(ALPHA Warning) Join a subnet as a validator",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node join command enables a Primary Network Validator to also be a validator
of a Subnet. If The command is run before the node is bootstrapped on the Primary Network, 
the command will fail. You can check the bootstrap status by calling 
avalanche node status`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         joinSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is validating")

	return cmd
}

func printJSONOutput(byteValue []byte) error {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, byteValue, "", "   "); err != nil {
		return err
	}
	ux.Logger.PrintToUser(prettyJSON.String())
	return nil
}

func parseBootstrappedOutput(filePath string, printOutput bool) (bool, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	err = json.Unmarshal(byteValue, &result)
	if err != nil {
		return false, err
	}
	if printOutput {
		err = printJSONOutput(byteValue)
		if err != nil {
			return false, err
		}
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

func parseSubnetSyncOutput(filePath string, printOutput bool) (string, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	err = json.Unmarshal(byteValue, &result)
	if err != nil {
		return "", err
	}
	if printOutput {
		err = printJSONOutput(byteValue)
		if err != nil {
			return "", err
		}
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
	err = json.Unmarshal(byteValue, &result)
	if err != nil {
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
	} else if weight < minValStake {
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
	_, _, err = deployer.AddValidatorPrimaryNetwork(nodeID, weight, start, duration, recipientAddr, uint32(20000))
	if err != nil {
		return err
	}
	return nil
}

func promptWeightPrimaryNetwork(network models.Network) (uint64, error) {
	defaultStake := constants.DefaultFujiPrimaryNetworkWeight
	defaultStakeStr := constants.DefaultFujiPrimaryNetworkWeightStr
	if network == models.Mainnet {
		defaultStake = constants.DefaultMainnetPrimaryNetworkWeight
		defaultStakeStr = constants.DefaultMainnetPrimaryNetworkWeightStr
	}
	defaultWeight := fmt.Sprintf("Default (%s)", defaultStakeStr)
	txt := "What stake weight would you like to assign to the validator?"
	weightOptions := []string{defaultWeight, "Custom"}
	weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
	if err != nil {
		return 0, err
	}

	switch weightOption {
	case defaultWeight:
		return uint64(defaultStake), nil
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

func checkNodeIsBootstrapped(clusterName string, printOutput bool) (bool, error) {
	ux.Logger.PrintToUser("Checking if node is bootstrapped to Primary Network ...")
	err := app.CreateFile(app.GetBootstrappedJSONFile())
	if err != nil {
		return false, err
	}
	if err := ansible.RunAnsiblePlaybookCheckBootstrapped(app.GetAnsibleDir(), app.GetBootstrappedJSONFile(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return false, err
	}
	isBootstrapped, err := parseBootstrappedOutput(app.GetBootstrappedJSONFile(), printOutput)
	if err != nil {
		return false, err
	}
	err = app.RemoveFile(app.GetBootstrappedJSONFile())
	if err != nil {
		return false, err
	}
	if isBootstrapped {
		return true, nil
	}
	ux.Logger.PrintToUser("Node is not bootstrapped yet, please check again later.")
	return false, nil
}

func getNodeID(clusterName string) (string, error) {
	ux.Logger.PrintToUser("Getting node id ...")
	err := app.CreateFile(app.GetNodeIDJSONFile())
	if err != nil {
		return "", err
	}
	if err := ansible.RunAnsiblePlaybookGetNodeID(app.GetAnsibleDir(), app.GetNodeIDJSONFile(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return "", err
	}
	nodeID, err := parseNodeIDOutput(app.GetNodeIDJSONFile())
	if err != nil {
		return "", err
	}
	err = app.RemoveFile(app.GetNodeIDJSONFile())
	if err != nil {
		return "", err
	}
	return nodeID, err
}

func getNodeSubnetSyncStatus(blockchainID, clusterName string, printOutput bool) (bool, error) {
	ux.Logger.PrintToUser("Checking if node is synced to subnet ...")
	err := app.CreateFile(app.GetSubnetSyncJSONFile())
	if err != nil {
		return false, err
	}
	if err := ansible.RunAnsiblePlaybookSubnetSyncStatus(app.GetAnsibleDir(), app.GetSubnetSyncJSONFile(), blockchainID, app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return false, err
	}
	subnetSyncStatus, err := parseSubnetSyncOutput(app.GetSubnetSyncJSONFile(), printOutput)
	if err != nil {
		return false, err
	}
	err = app.RemoveFile(app.GetSubnetSyncJSONFile())
	if err != nil {
		return false, err
	}
	if subnetSyncStatus == constants.NodeIsSubnetSynced {
		return true, nil
	} else if subnetSyncStatus == constants.NodeIsSubnetValidating {
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
	err := checkNodeIsPrimaryNetworkValidator(nodeID, network)
	if err == nil {
		err = validatePrimaryNetwork(nodeID, network)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Node successfully added as Primary Network validator!")
	}
	return nil
}

func waitForNodeToBePrimaryNetworkValidator(nodeID ids.NodeID) {
	ux.Logger.PrintToUser("Waiting 1 min for the node to be a Primary Network Validator...")
	// wait for 60 seconds because we set the start time to in 1 minute
	time.Sleep(60 * time.Second)
	// long polling: try up to 5 times
	for i := 0; i < 5; i++ {
		// checkNodeIsPrimaryNetworkValidator only returns err if node is already a Primary Network validator
		err := checkNodeIsPrimaryNetworkValidator(nodeID, models.Fuji)
		if err != nil {
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
	isBootstrapped, err := checkNodeIsBootstrapped(clusterName, false)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	nodeIDStr, err := getNodeID(clusterName)
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
	_, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
	if err != nil {
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
	isSubnetSynced, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, false)
	if err != nil {
		return err
	}
	if !isSubnetSynced {
		return errors.New("node is not synced to subnet yet, please try again later")
	}
	err = addNodeAsPrimaryNetworkValidator(nodeID, models.Fuji)
	if err != nil {
		return err
	}
	waitForNodeToBePrimaryNetworkValidator(nodeID)
	err = addNodeAsSubnetValidator(nodeIDStr, models.Fuji)
	if err != nil {
		return err
	}
	return nil
}

// convertToAVAX converts nanoAVAX to AVAX
func convertToAVAX(weight uint64) string {
	return fmt.Sprintf("%.9f %s", float64(weight)/float64(units.Avax), constants.AVAX)
}

func printNodeJoinOutput(nodeID ids.NodeID, network models.Network, start time.Time) {
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(duration).Format(constants.TimeParseLayout))
	// we need to divide by 10 ^ 9 since we were using nanoAvax
	ux.Logger.PrintToUser("Weight: %s", convertToAVAX(weight))
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")
}

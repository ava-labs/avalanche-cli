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

	"github.com/ava-labs/avalanchego/genesis"
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

func newValidatePrimaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "primary [clusterName]",
		Short: "(ALPHA Warning) Join Primary Network as a validator",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node validate primary command enables all nodes in a cluster to be validators of Primary
Network.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         validatePrimaryNetwork,
	}
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "set up validator in testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "set up validator in fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "set up validator in mainnet")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().Uint64Var(&weight, "stake-amount", 0, "how many AVAX to stake in the validator")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")

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

func parseBootstrappedOutput(filePath, hostAlias string, printOutput bool) (bool, error) {
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
	if printOutput {
		ux.Logger.PrintToUser(fmt.Sprintf("Bootstrap status for node %s:", hostAlias))
		if err = printJSONOutput(byteValue); err != nil {
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
	return false, errors.New("unable to parse node bootstrap status")
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
	return "", errors.New("unable to parse node ID")
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

func joinAsPrimaryNetworkValidator(nodeID ids.NodeID, network models.Network) error {
	ux.Logger.PrintToUser(fmt.Sprintf("Adding node %s as a Primary Network Validator...", nodeID.String()))
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
	printNodeJoinPrimaryNetworkOutput(nodeID, network, start)
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
	defaultWeight := fmt.Sprintf("Default (%s)", convertNanoAvaxToAvaxString(defaultStake))
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
	start := time.Now().Add(constants.PrimaryNetworkValidatingStartLeadTime)
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

func checkClusterIsBootstrapped(clusterName string, printOutput bool) ([]string, error) {
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
	if err != nil {
		return nil, err
	}
	notBootstrappedNodes := []string{}
	ux.Logger.PrintToUser(fmt.Sprintf("Checking if node(s) in cluster %s are bootstrapped to Primary Network ...", clusterName))
	for _, host := range hostAliases {
		if err := app.CreateAnsibleStatusFile(app.GetBootstrappedJSONFile()); err != nil {
			return nil, err
		}
		if err := ansible.RunAnsiblePlaybookCheckBootstrapped(app.GetAnsibleDir(), app.GetBootstrappedJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			return nil, err
		}
		isBootstrapped, err := parseBootstrappedOutput(app.GetBootstrappedJSONFile(), host, printOutput)
		if err != nil {
			return nil, err
		}
		if err := app.RemoveAnsibleStatusDir(); err != nil {
			return nil, err
		}
		if !isBootstrapped {
			notBootstrappedNodes = append(notBootstrappedNodes, host)
		}
	}
	return notBootstrappedNodes, nil
}

func getClusterNodeID(clusterName, hostAlias string) (string, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Getting Avalanche node id for node %s...", hostAlias))
	if err := app.CreateAnsibleStatusFile(app.GetNodeIDJSONFile()); err != nil {
		return "", err
	}
	if err := ansible.RunAnsiblePlaybookGetNodeID(app.GetAnsibleDir(), app.GetNodeIDJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), hostAlias); err != nil {
		return "", err
	}
	nodeID, err := parseNodeIDOutput(app.GetNodeIDJSONFile())
	if err != nil {
		return "", err
	}
	if err = app.RemoveAnsibleStatusDir(); err != nil {
		return "", err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Avalanche node id is %s", nodeID))
	return nodeID, err
}

// checkNodeIsPrimaryNetworkValidator only returns err if node is already a Primary Network validator
func checkNodeIsPrimaryNetworkValidator(nodeID ids.NodeID, network models.Network) (bool, error) {
	isValidator, err := subnet.IsSubnetValidator(ids.Empty, nodeID, network)
	if err != nil {
		return false, err
	}
	return isValidator, nil
}

// addNodeAsPrimaryNetworkValidator returns bool if node is added as primary network validator
// as it impacts the output in adding node as subnet validator in the next steps
func addNodeAsPrimaryNetworkValidator(nodeID ids.NodeID, network models.Network) (bool, error) {
	isValidator, err := checkNodeIsPrimaryNetworkValidator(nodeID, network)
	if err != nil {
		return false, err
	}
	if !isValidator {
		if err = joinAsPrimaryNetworkValidator(nodeID, network); err != nil {
			return false, err
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Node %s successfully added as Primary Network validator!", nodeID.String()))
		return true, nil
	}
	return false, nil
}

func validatePrimaryNetwork(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	err := setupAnsible()
	if err != nil {
		return err
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName, false)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
	if err != nil {
		return err
	}
	failedNodes := []string{}
	nodeErrors := []error{}
	for _, host := range hostAliases {
		nodeIDStr, err := getClusterNodeID(clusterName, host)
		if err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		_, err = addNodeAsPrimaryNetworkValidator(nodeID, models.Fuji)
		if err != nil {
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
		}
	}
	if len(failedNodes) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for i, node := range failedNodes {
			ux.Logger.PrintToUser(fmt.Sprintf("node %s failed due to %s", node, nodeErrors[i]))
		}
		return fmt.Errorf("node(s) %s failed to validate the Primary Network", failedNodes)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are successfully added as Primary Network validators!"), clusterName)
	}
	return nil
}

// convertNanoAvaxToAvaxString converts nanoAVAX to AVAX
func convertNanoAvaxToAvaxString(weight uint64) string {
	return fmt.Sprintf("%.2f %s", float64(weight)/float64(units.Avax), constants.AVAXSymbol)
}

func printNodeJoinPrimaryNetworkOutput(nodeID ids.NodeID, network models.Network, start time.Time) {
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.String())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(duration).Format(constants.TimeParseLayout))
	// we need to divide by 10 ^ 9 since we were using nanoAvax
	ux.Logger.PrintToUser("Weight: %s", convertNanoAvaxToAvaxString(weight))
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")
}

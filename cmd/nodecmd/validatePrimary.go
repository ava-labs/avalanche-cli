// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"golang.org/x/exp/maps"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"

	"github.com/ava-labs/avalanchego/vms/platformvm"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	deployDevnet                 bool
	deployTestnet                bool
	deployMainnet                bool
	endpoint                     string
	keyName                      string
	useEwoq                      bool
	useLedger                    bool
	useStaticIP                  bool
	sameMonitoringInstance       bool
	separateMonitoringInstance   bool
	awsProfile                   string
	ledgerAddresses              []string
	weight                       uint64
	startTimeStr                 string
	duration                     time.Duration
	defaultValidatorParams       bool
	useCustomDuration            bool
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

	cmd.Flags().BoolVarP(&deployDevnet, "devnet", "d", false, "set up validator in devnet")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "set up validator in testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "set up validator in fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&deployMainnet, "mainnet", "m", false, "set up validator in mainnet")

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")

	cmd.Flags().Uint64Var(&weight, "stake-amount", 0, "how many AVAX to stake in the validator")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")

	return cmd
}

func GetMinStakingAmount(network models.Network) (uint64, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	minValStake, _, err := pClient.GetMinStake(ctx, ids.Empty)
	if err != nil {
		return 0, err
	}
	return minValStake, nil
}

func joinAsPrimaryNetworkValidator(
	network models.Network,
	kc *keychain.Keychain,
	nodeID ids.NodeID,
	nodeIndex int,
	signingKeyPath string,
	nodeCmd bool,
) error {
	ux.Logger.PrintToUser(fmt.Sprintf("Adding node %s as a Primary Network Validator...", nodeID.String()))
	var (
		start time.Time
		err   error
	)
	minValStake, err := GetMinStakingAmount(network)
	if err != nil {
		return err
	}
	if weight == 0 {
		weight, err = PromptWeightPrimaryNetwork(network)
		if err != nil {
			return err
		}
	}
	if weight < minValStake {
		return fmt.Errorf("illegal weight, must be greater than or equal to %d: %d", minValStake, weight)
	}
	start, duration, err = GetTimeParametersPrimaryNetwork(network, nodeIndex, duration, startTimeStr, nodeCmd)
	if err != nil {
		return err
	}

	recipientAddr := kc.Addresses().List()[0]
	deployer := subnet.NewPublicDeployer(app, kc, network)
	PrintNodeJoinPrimaryNetworkOutput(nodeID, weight, network, start)
	// we set the starting time for node to be a Primary Network Validator to be in 1 minute
	// we use min delegation fee as default
	delegationFee := network.GenesisParams().MinDelegationFee
	blsKeyBytes, err := os.ReadFile(signingKeyPath)
	if err != nil {
		return err
	}
	blsSk, err := bls.SecretKeyFromBytes(blsKeyBytes)
	if err != nil {
		return err
	}
	_, err = deployer.AddPermissionlessValidator(
		ids.Empty,
		ids.Empty,
		nodeID,
		weight,
		uint64(start.Unix()),
		uint64(start.Add(duration).Unix()),
		recipientAddr,
		delegationFee,
		nil,
		signer.NewProofOfPossession(blsSk),
	)
	return err
}

func PromptWeightPrimaryNetwork(network models.Network) (uint64, error) {
	defaultStake := network.GenesisParams().MinValidatorStake
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

func GetTimeParametersPrimaryNetwork(network models.Network, nodeIndex int, validationDuration time.Duration, validationStartTimeStr string, nodeCmd bool) (time.Time, time.Duration, error) {
	const (
		defaultDurationOption = "Minimum staking duration on primary network"
		custom                = "Custom"
	)
	var err error
	var start time.Time
	if validationStartTimeStr != "" {
		start, err = time.Parse(constants.TimeParseLayout, validationStartTimeStr)
		if err != nil {
			return time.Time{}, 0, err
		}
	} else {
		start = time.Now().Add(constants.PrimaryNetworkValidatingStartLeadTimeNodeCmd)
		if !nodeCmd {
			start = time.Now().Add(constants.PrimaryNetworkValidatingStartLeadTime)
		}
	}
	if useCustomDuration && validationDuration != 0 {
		return start, duration, nil
	}
	if validationDuration != 0 {
		duration, err = getDefaultValidationTime(start, network, nodeIndex)
		if err != nil {
			return time.Time{}, 0, err
		}
		return start, duration, nil
	}
	msg := "How long should your validator validate for?"
	durationOptions := []string{defaultDurationOption, custom}
	durationOption, err := app.Prompt.CaptureList(msg, durationOptions)
	if err != nil {
		return time.Time{}, 0, err
	}
	switch durationOption {
	case defaultDurationOption:
		duration, err = getDefaultValidationTime(start, network, nodeIndex)
		if err != nil {
			return time.Time{}, 0, err
		}
	default:
		useCustomDuration = true
		duration, err = subnetcmd.PromptDuration(start, network)
		if err != nil {
			return time.Time{}, 0, err
		}
	}
	return start, duration, nil
}

func getDefaultValidationTime(start time.Time, network models.Network, nodeIndex int) (time.Duration, error) {
	durationStr := constants.DefaultFujiStakeDuration
	if network.Kind == models.Mainnet {
		durationStr = constants.DefaultMainnetStakeDuration
	}
	durationInt, err := strconv.Atoi(durationStr[:len(durationStr)-1])
	if err != nil {
		return 0, err
	}
	// stagger expiration time by 1 day for each added node
	durationAddition := 24 * nodeIndex
	durationStr = strconv.Itoa(durationInt+durationAddition) + "h"
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}
	end := start.Add(d)
	if nodeIndex == 0 {
		confirm := fmt.Sprintf("Your validator will finish staking by %s", end.Format(constants.TimeParseLayout))
		yes, err := app.Prompt.CaptureYesNo(confirm)
		if err != nil {
			return 0, err
		}
		if !yes {
			return 0, errors.New("you have to confirm staking duration")
		}
	}
	return d, nil
}

func getNodeIDs(hosts []*models.Host) (map[string]string, map[string]error) {
	nodeIDMap := map[string]string{}
	failedNodes := map[string]error{}
	for _, host := range hosts {
		cloudNodeID := host.GetCloudID()
		nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudNodeID))
		if err != nil {
			failedNodes[host.NodeID] = err
			continue
		}
		ux.Logger.PrintToUser("Avalanche node id for host %s is %s", host.NodeID, nodeID)
		nodeIDMap[host.NodeID] = nodeID.String()
	}
	return nodeIDMap, failedNodes
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
func addNodeAsPrimaryNetworkValidator(
	network models.Network,
	kc *keychain.Keychain,
	nodeID ids.NodeID,
	nodeIndex int,
	instanceID string,
) (bool, error) {
	isValidator, err := checkNodeIsPrimaryNetworkValidator(nodeID, network)
	if err != nil {
		return false, err
	}
	if !isValidator {
		signingKeyPath := app.GetNodeBLSSecretKeyPath(instanceID)
		if err = joinAsPrimaryNetworkValidator(network, kc, nodeID, nodeIndex, signingKeyPath, true); err != nil {
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

	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}

	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	cluster, ok := clustersConfig.Clusters[clusterName]
	if !ok {
		return fmt.Errorf("cluster %s does not exist", clusterName)
	}
	network := clustersConfig.Clusters[clusterName].Network
	hosts := cluster.GetValidatorHosts(allHosts) // exlude api nodes
	defer disconnectHosts(hosts)

	fee := network.GenesisParams().AddPrimaryNetworkValidatorFee * uint64(len(hosts))
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

	notBootstrappedNodes, err := checkHostsAreBootstrapped(hosts)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	notHealthyNodes, err := checkHostsAreHealthy(hosts)
	if err != nil {
		return err
	}
	if len(notHealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy, please fix the issue and again", notHealthyNodes)
	}
	ux.Logger.PrintToUser("Note that we have staggered the end time of validation period to increase by 24 hours for each node added if multiple nodes are added as Primary Network validators simultaneously")
	nodeIDMap, failedNodesMap := getNodeIDs(hosts)
	nodeErrors := map[string]error{}
	for i, host := range hosts {
		nodeIDStr, b := nodeIDMap[host.NodeID]
		if !b {
			err, b := failedNodesMap[host.NodeID]
			if !b {
				return fmt.Errorf("expected to found an error for non mapped node")
			}
			ux.Logger.PrintToUser("Failed to add node %s as Primary Network validator due to %s", host.NodeID, err)
			nodeErrors[host.NodeID] = err
			continue
		}
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			ux.Logger.PrintToUser("Failed to add node %s as Primary Network validator due to %s", host.NodeID, err)
			nodeErrors[host.NodeID] = err
			continue
		}
		_, clusterNodeID, err := models.HostAnsibleIDToCloudID(host.NodeID)
		if err != nil {
			ux.Logger.PrintToUser("Failed to add node %s as Primary Network due to %s", host.NodeID, err.Error())
			nodeErrors[host.NodeID] = err
			continue
		}
		_, err = addNodeAsPrimaryNetworkValidator(network, kc, nodeID, i, clusterNodeID)
		if err != nil {
			ux.Logger.PrintToUser("Failed to add node %s as Primary Network validator due to %s", host.NodeID, err)
			nodeErrors[host.NodeID] = err
		}
	}
	if len(nodeErrors) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for node, nodeErr := range nodeErrors {
			ux.Logger.PrintToUser("node %s failed due to %v", node, nodeErr)
		}
		return fmt.Errorf("node(s) %s failed to validate the Primary Network", maps.Keys(nodeErrors))
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are successfully added as Primary Network validators!", clusterName))
	}
	return nil
}

// convertNanoAvaxToAvaxString converts nanoAVAX to AVAX
func convertNanoAvaxToAvaxString(weight uint64) string {
	return fmt.Sprintf("%.2f %s", float64(weight)/float64(units.Avax), constants.AVAXSymbol)
}

func PrintNodeJoinPrimaryNetworkOutput(nodeID ids.NodeID, weight uint64, network models.Network, start time.Time) {
	ux.Logger.PrintToUser("NodeID: %s", nodeID.String())
	ux.Logger.PrintToUser("Network: %s", network.Name())
	ux.Logger.PrintToUser("Start time: %s", start.Format(constants.TimeParseLayout))
	ux.Logger.PrintToUser("End time: %s", start.Add(duration).Format(constants.TimeParseLayout))
	// we need to divide by 10 ^ 9 since we were using nanoAvax
	ux.Logger.PrintToUser("Weight: %s", convertNanoAvaxToAvaxString(weight))
	ux.Logger.PrintToUser("Inputs complete, issuing transaction to add the provided validator information...")
}

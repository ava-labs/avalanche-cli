// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	blockchaincmd "github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

var avoidSubnetValidationChecks bool

func newValidateSubnetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subnet [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Join a Subnet as a validator",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node validate subnet command enables all nodes in a cluster to be validators of a Subnet.
If the command is run before the nodes are Primary Network validators, the command will first
make the nodes Primary Network validators before making them Subnet validators. 
If The command is run before the nodes are bootstrapped on the Primary Network, the command will fail. 
You can check the bootstrap status by calling avalanche node status <clusterName>
If The command is run before the nodes are synced to the subnet, the command will fail.
You can check the subnet sync status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		Args: cobrautils.ExactArgs(2),
		RunE: validateSubnet,
	}

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")

	cmd.Flags().Uint64Var(&weight, "stake-amount", 0, "how many AVAX to stake in the validator")
	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long validator validates for after start time")
	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "use default weight/start/duration params for subnet validator")

	cmd.Flags().StringSliceVar(&validators, "validators", []string{}, "validate subnet for the given comma separated list of validators. defaults to all cluster nodes")

	cmd.Flags().BoolVar(&avoidSubnetValidationChecks, "no-validation-checks", true, "do not check if subnet is already synced or validated")
	cmd.Flags().BoolVar(&avoidChecks, "no-checks", false, "do not check for bootstrapped status or healthy status")

	return cmd
}

func parseSubnetSyncOutput(byteValue []byte) (string, error) {
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
	return "", errors.New("unable to parse subnet sync status")
}

func addNodeAsSubnetValidator(
	deployer *subnet.PublicDeployer,
	network models.Network,
	subnetID ids.ID,
	kc *keychain.Keychain,
	useLedger bool,
	nodeID string,
	subnetName string,
	currentNodeIndex int,
	nodeCount int,
) error {
	// devnet criteria: as per tests with RD env
	waitForTxAcceptance := false
	waitForValidatorInCurrentList := true
	if network.Kind != models.Devnet {
		// fuji criteria: current validators seems to be pretty slow to update in fuji
		waitForTxAcceptance = true
		waitForValidatorInCurrentList = false
	}
	ux.Logger.PrintToUser("Adding the node as a Subnet Validator...")
	defer ux.Logger.PrintLineSeparator()
	if err := blockchaincmd.CallAddValidatorNonSOV(
		deployer,
		network,
		kc,
		useLedger,
		subnetName,
		nodeID,
		defaultValidatorParams,
		waitForTxAcceptance,
	); err != nil {
		return err
	}
	if waitForValidatorInCurrentList {
		if err := waitForSubnetValidator(network, subnetID, nodeID); err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Node %s successfully added as Subnet validator! (%d / %d)", nodeID, currentNodeIndex+1, nodeCount)
	return nil
}

func waitForSubnetValidator(
	network models.Network,
	subnetID ids.ID,
	nodeIDStr string,
) error {
	timeout := 5 * time.Minute
	poolTime := 1 * time.Second
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	startTime := time.Now()
	for {
		isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
		if err != nil {
			return err
		}
		if isValidator {
			return nil
		}
		if time.Since(startTime) > timeout {
			return fmt.Errorf("node %s not validating subnet ID %s after %d seconds", nodeID, subnetID, uint32(timeout.Seconds()))
		}
		time.Sleep(poolTime)
	}
}

// getNodeSubnetSyncStatus checks if node is bootstrapped to blockchain blockchainID
// if getNodeSubnetSyncStatus is called from node validate subnet command, it will fail if
// node status is not 'syncing'. If getNodeSubnetSyncStatus is called from node status command,
// it will return true node status is 'syncing'
func getNodeSubnetSyncStatus(
	host *models.Host,
	blockchainID string,
) (string, error) {
	ux.Logger.PrintToUser("Checking if node %s is synced to subnet ...", host.NodeID)
	if resp, err := ssh.RunSSHSubnetSyncStatus(host, blockchainID); err != nil {
		return "", err
	} else {
		if subnetSyncStatus, err := parseSubnetSyncOutput(resp); err != nil {
			return "", err
		} else {
			return subnetSyncStatus, nil
		}
	}
}

func waitForNodeToBePrimaryNetworkValidator(network models.Network, nodeID ids.NodeID) error {
	ux.Logger.PrintToUser("Waiting for the node %s to start as a Primary Network Validator...", nodeID)
	return waitForSubnetValidator(network, ids.Empty, nodeID.String())
}

func validateSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]

	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	if _, err := blockchaincmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}

	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if clusterConfig.Local {
		return notImplementedForLocal("validate subnet")
	}
	network := clusterConfig.Network

	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts := clusterConfig.GetValidatorHosts(allHosts) // exlude api nodes
	if len(validators) != 0 {
		hosts, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}
	defer node.DisconnectHosts(hosts)

	nodeIDMap, failedNodesMap := getNodeIDs(hosts)
	nonPrimaryValidators := 0
	for hostNodeID, nodeID := range nodeIDMap {
		isValidator, err := checkNodeIsPrimaryNetworkValidator(nodeID, network)
		if err != nil {
			ux.Logger.PrintToUser("Failed to verify if node %s is a primary network validator due to %s", hostNodeID, err)
			continue
		}
		if !isValidator {
			nonPrimaryValidators++
		}
	}
	// TODO: will estimate fee in subsecuent PR
	// AddPrimaryNetworkValidatorFee*uint64(nonPrimaryValidators) + AddSubnetValidatorFee*uint64(len(hosts))
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
	if err := blockchaincmd.UpdateKeychainWithSubnetControlKeys(kc, network, subnetName); err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)

	if !avoidChecks {
		if err := node.CheckHostsAreBootstrapped(hosts); err != nil {
			return err
		}
		if err := node.CheckHostsAreHealthy(hosts); err != nil {
			return err
		}
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	var blockchainID ids.ID
	if !avoidSubnetValidationChecks {
		blockchainID = sc.Networks[network.Name()].BlockchainID
		if blockchainID == ids.Empty {
			return constants.ErrNoBlockchainID
		}
	}
	nodeErrors := map[string]error{}
	// set node errors for node ID conversions
	for _, host := range hosts {
		if _, b := nodeIDMap[host.NodeID]; !b {
			if err, b := failedNodesMap[host.NodeID]; !b {
				return fmt.Errorf("expected to found an error for non mapped node")
			} else {
				ux.Logger.PrintToUser("Failed to add node %s as subnet validator due to %s", host.NodeID, err)
				nodeErrors[host.NodeID] = err
			}
		}
	}
	if nonPrimaryValidators > 0 {
		// add primary validators loop
		for i, host := range hosts {
			if _, ok := nodeErrors[host.NodeID]; ok {
				continue
			}
			nodeID, b := nodeIDMap[host.NodeID]
			if !b {
				return fmt.Errorf("nodeID should be defined on add primary validators")
			}
			if err := addNodeAsPrimaryNetworkValidator(deployer, network, kc, nodeID, i, host.GetCloudID()); err != nil {
				ux.Logger.PrintToUser("Failed to add node %s as subnet validator due to %s", host.NodeID, err.Error())
				nodeErrors[host.NodeID] = err
				continue
			}
		}
		// wait primary validators loop
		for _, host := range hosts {
			if _, ok := nodeErrors[host.NodeID]; ok {
				continue
			}
			nodeID, b := nodeIDMap[host.NodeID]
			if !b {
				return fmt.Errorf("nodeID should be defined on primary validators wait loop")
			}
			if err := waitForNodeToBePrimaryNetworkValidator(network, nodeID); err != nil {
				ux.Logger.PrintToUser("Failed to add node %s as subnet validator due to %s", host.NodeID, err.Error())
				nodeErrors[host.NodeID] = err
				continue
			}
		}
	}
	// add subnet validators loop
	for i, host := range hosts {
		if _, ok := nodeErrors[host.NodeID]; ok {
			continue
		}
		nodeID, b := nodeIDMap[host.NodeID]
		if !b {
			return fmt.Errorf("nodeID should be defined on add subnet validators loop")
		}
		if !avoidSubnetValidationChecks {
			// we have to check if node is synced to subnet before adding the node as a validator
			subnetSyncStatus, err := getNodeSubnetSyncStatus(host, blockchainID.String())
			if err != nil {
				ux.Logger.PrintToUser("Failed to get subnet sync status for node %s", host.NodeID)
				nodeErrors[host.NodeID] = err
				continue
			}
			if subnetSyncStatus != status.Syncing.String() {
				if subnetSyncStatus == status.Validating.String() {
					ux.Logger.PrintToUser("Failed to add node %s as subnet validator as node is already a subnet validator", host.NodeID)
					nodeErrors[host.NodeID] = errors.New("node is already a subnet validator")
				} else {
					ux.Logger.PrintToUser("Failed to add node %s as subnet validator as node is not synced to subnet yet", host.NodeID)
					nodeErrors[host.NodeID] = errors.New("node is not synced to subnet yet, please try again later")
				}
				continue
			}
		}
		if isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network); err != nil {
			ux.Logger.PrintToUser("Failed to get validator status for node %s", host.NodeID)
			nodeErrors[host.NodeID] = err
			continue
		} else if isValidator {
			ux.Logger.PrintToUser("Failed to add node %s as subnet validator as node is already a subnet validator", host.NodeID)
			nodeErrors[host.NodeID] = errors.New("node is already a subnet validator")
			continue
		}
		if err := addNodeAsSubnetValidator(deployer, network, subnetID, kc, useLedger, nodeID.String(), subnetName, i, len(hosts)); err != nil {
			ux.Logger.PrintToUser("Failed to add node %s as subnet validator due to %s", host.NodeID, err.Error())
			nodeErrors[host.NodeID] = err
		}
	}
	if len(nodeErrors) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for node, err := range nodeErrors {
			ux.Logger.PrintToUser("node %s failed due to %s", node, err)
		}
		return fmt.Errorf("node(s) %s failed to validate subnet %s", maps.Keys(nodeErrors), subnetName)
	} else {
		ux.Logger.PrintToUser("All nodes in cluster %s are successfully added as Subnet validators!", clusterName)
	}
	return nil
}

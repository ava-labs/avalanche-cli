// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/vms/platformvm/status"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

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
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         validateSubnet,
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
	return "", errors.New("unable to parse subnet sync status")
}

func addNodeAsSubnetValidator(nodeID string, network models.Network, currentNodeIndex, nodeCount int) error {
	ux.Logger.PrintToUser("Adding the node as a Subnet Validator...")
	if err := subnetcmd.CallAddValidator(subnetName, nodeID, network); err != nil {
		return err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Node %s successfully added as Subnet validator! (%d / %d)", nodeID, currentNodeIndex+1, nodeCount))
	ux.Logger.PrintToUser("======================================")
	return nil
}

func getNodeSubnetSyncStatus(blockchainID, clusterName, hostAlias string, statusOutput, errOnValidating bool) (bool, error) {
	if statusOutput {
		ux.Logger.PrintToUser(fmt.Sprintf("Checking if node %s is synced to subnet ...", hostAlias))
	} else {
		ux.Logger.PrintToUser("Checking if node is synced to subnet ...")
	}
	if err := app.CreateAnsibleStatusFile(app.GetSubnetSyncJSONFile()); err != nil {
		return false, err
	}
	if err := ansible.RunAnsiblePlaybookSubnetSyncStatus(app.GetAnsibleDir(), app.GetSubnetSyncJSONFile(), blockchainID, app.GetAnsibleInventoryDirPath(clusterName), hostAlias); err != nil {
		return false, err
	}
	subnetSyncStatus, err := parseSubnetSyncOutput(app.GetSubnetSyncJSONFile())
	if err != nil {
		return false, err
	}
	if err = app.RemoveAnsibleStatusDir(); err != nil {
		return false, err
	}
	// if function is called from status command
	if statusOutput {
		if subnetSyncStatus == status.Validating.String() {
			return true, nil
		}
		return false, nil
	}
	if subnetSyncStatus == status.Syncing.String() {
		return true, nil
	} else if subnetSyncStatus == status.Validating.String() && errOnValidating {
		return false, errors.New("node is already a subnet validator")
	}
	return false, nil
}

func waitForNodeToBePrimaryNetworkValidator(nodeID ids.NodeID) error {
	ux.Logger.PrintToUser("Waiting for the node to start as a Primary Network Validator...")
	// wait for 20 seconds because we set the start time to be in 20 seconds
	time.Sleep(20 * time.Second)
	// long polling: try up to 5 times
	for i := 0; i < 5; i++ {
		isValidator, err := checkNodeIsPrimaryNetworkValidator(nodeID, models.Fuji)
		if err != nil {
			return err
		}
		if isValidator {
			break
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func validateSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName = args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	err := setupAnsible()
	if err != nil {
		return err
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
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
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
	if err != nil {
		return err
	}
	failedNodes := []string{}
	nodeErrors := []error{}
	for i, host := range hostAliases {
		nodeIDStr, err := getClusterNodeID(clusterName, host)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator due to %s", host, err.Error()))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator due to %s", host, err.Error()))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		// we have to check if node is synced to subnet before adding the node as a validator
		isSubnetSynced, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, host, false, true)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to get subnet sync status for node %s", host))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		if !isSubnetSynced {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator as node is not synced to subnet yet", host))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, errors.New("node is not synced to subnet yet, please try again later"))
			continue
		}
		addedNodeAsPrimaryNetworkValidator, err := addNodeAsPrimaryNetworkValidator(nodeID, models.Fuji, i)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator due to %s", host, err.Error()))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		if addedNodeAsPrimaryNetworkValidator {
			if err := waitForNodeToBePrimaryNetworkValidator(nodeID); err != nil {
				ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator due to %s", host, err.Error()))
				failedNodes = append(failedNodes, host)
				nodeErrors = append(nodeErrors, err)
				continue
			}
		}
		err = addNodeAsSubnetValidator(nodeIDStr, models.Fuji, i, len(hostAliases))
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to add node %s as subnet validator due to %s", host, err.Error()))
			failedNodes = append(failedNodes, host)
			nodeErrors = append(nodeErrors, err)
		}
	}
	if len(failedNodes) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for i, node := range failedNodes {
			ux.Logger.PrintToUser(fmt.Sprintf("node %s failed due to %s", node, nodeErrors[i]))
		}
		return fmt.Errorf("node(s) %s failed to validate subnet %s", failedNodes, subnetName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are successfully added as Subnet validators!", clusterName))
	}
	return nil
}

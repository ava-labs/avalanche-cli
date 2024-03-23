// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/olekukonko/tablewriter"
	"github.com/pborman/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var subnetName string

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [clusterName]",
		Short: "(ALPHA Warning) Get node bootstrap status",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node status command gets the bootstrap status of all nodes in a cluster with the Primary Network. 
If no cluster is given, defaults to node list behaviour.

To get the bootstrap status of a node with a Subnet, use --subnet flag`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(0),
		RunE:         statusNode,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is syncing with")

	return cmd
}

func statusNode(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return list(nil, nil)
	}
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	var blockchainID ids.ID
	if subnetName != "" {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		blockchainID = sc.Networks[clusterConf.Network.Name()].BlockchainID
		if blockchainID == ids.Empty {
			return ErrNoBlockchainID
		}
	}
	hostIDs := utils.Filter(clusterConf.GetCloudIDs(), clusterConf.IsAvalancheGoHost)
	nodeIDs, err := utils.MapWithError(hostIDs, func(s string) (string, error) {
		n, err := getNodeID(app.GetNodeInstanceDirPath(s))
		return n.String(), err
	})
	if err != nil {
		return err
	}
	if subnetName != "" {
		// check subnet first
		if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
			return err
		}
	}

	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer disconnectHosts(hosts)

	notBootstrappedNodes, err := getNotBootstrappedNodes(hosts)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Checking if node(s) are healthy...")
	unhealthyNodes, err := getUnhealthyNodes(hosts)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Getting avalanchego version of node(s)...")

	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckAvalancheGoVersion(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if avalancheGoVersion, _, err := parseAvalancheGoOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), avalancheGoVersion, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return fmt.Errorf("failed to get avalanchego version for node(s) %s", wgResults.GetErrorHostMap())
	}
	avagoVersions := map[string]string{}
	for nodeID, avalanchegoVersion := range wgResults.GetResultMap() {
		avagoVersions[nodeID] = fmt.Sprintf("%v", avalanchegoVersion)
	}

	notSyncedNodes := []string{}
	subnetSyncedNodes := []string{}
	subnetValidatingNodes := []string{}
	if subnetName != "" {
		hostsToCheckSyncStatus := []string{}
		for _, hostID := range hostIDs {
			if slices.Contains(notBootstrappedNodes, hostID) {
				notSyncedNodes = append(notSyncedNodes, hostID)
			} else {
				hostsToCheckSyncStatus = append(hostsToCheckSyncStatus, hostID)
			}
		}
		if len(hostsToCheckSyncStatus) != 0 {
			ux.Logger.PrintToUser("Getting subnet sync status of node(s)")
			hostsToCheck := utils.Filter(hosts, func(h *models.Host) bool { return slices.Contains(hostsToCheckSyncStatus, h.GetCloudID()) })
			wg := sync.WaitGroup{}
			wgResults := models.NodeResults{}
			for _, host := range hostsToCheck {
				wg.Add(1)
				go func(nodeResults *models.NodeResults, host *models.Host) {
					defer wg.Done()
					if syncstatus, err := ssh.RunSSHSubnetSyncStatus(host, blockchainID.String()); err != nil {
						nodeResults.AddResult(host.GetCloudID(), nil, err)
						return
					} else {
						if subnetSyncStatus, err := parseSubnetSyncOutput(syncstatus); err != nil {
							nodeResults.AddResult(host.GetCloudID(), nil, err)
							return
						} else {
							nodeResults.AddResult(host.GetCloudID(), subnetSyncStatus, err)
						}
					}
				}(&wgResults, host)
			}
			wg.Wait()
			if wgResults.HasErrors() {
				return fmt.Errorf("failed to check sync status for node(s) %s", wgResults.GetErrorHostMap())
			}
			for nodeID, subnetSyncStatus := range wgResults.GetResultMap() {
				switch subnetSyncStatus {
				case status.Syncing.String():
					subnetSyncedNodes = append(subnetSyncedNodes, nodeID)
				case status.Validating.String():
					subnetValidatingNodes = append(subnetValidatingNodes, nodeID)
				default:
					notSyncedNodes = append(notSyncedNodes, nodeID)
				}
			}
		}
	}
	if clusterConf.MonitoringInstance != "" {
		hostIDs = append(hostIDs, clusterConf.MonitoringInstance)
		nodeIDs = append(nodeIDs, "")
	}
	nodeConfigs := []models.NodeConfig{}
	for _, hostID := range hostIDs {
		nodeConfig, err := app.LoadClusterNodeConfig(hostID)
		if err != nil {
			return err
		}
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}
	printOutput(
		clusterConf,
		hostIDs,
		nodeIDs,
		avagoVersions,
		unhealthyNodes,
		notBootstrappedNodes,
		notSyncedNodes,
		subnetSyncedNodes,
		subnetValidatingNodes,
		clusterName,
		subnetName,
		nodeConfigs,
	)
	return nil
}

func printOutput(
	clusterConf models.ClusterConfig,
	cloudIDs []string,
	nodeIDs []string,
	avagoVersions map[string]string,
	unhealthyHosts []string,
	notBootstrappedHosts []string,
	notSyncedHosts []string,
	subnetSyncedHosts []string,
	subnetValidatingHosts []string,
	clusterName string,
	subnetName string,
	nodeConfigs []models.NodeConfig,
) {
	if subnetName == "" && len(notBootstrappedHosts) == 0 {
		ux.Logger.PrintToUser("All nodes in cluster %s are bootstrapped to Primary Network!", clusterName)
	}
	if subnetName != "" && len(notSyncedHosts) == 0 {
		// all nodes are either synced to or validating subnet
		status := "synced to"
		if len(subnetSyncedHosts) == 0 {
			status = "validators of"
		}
		ux.Logger.PrintToUser("All nodes in cluster %s are %s Subnet %s", logging.LightBlue.Wrap(clusterName), status, subnetName)
	}
	ux.Logger.PrintToUser("")
	tit := fmt.Sprintf("STATUS FOR CLUSTER: %s", logging.LightBlue.Wrap(clusterName))
	ux.Logger.PrintToUser(tit)
	ux.Logger.PrintToUser(strings.Repeat("=", len(removeColors(tit))))
	ux.Logger.PrintToUser("")
	header := []string{"Cloud ID", "Node ID", "IP", "Network", "Role", "Avago Version", "Primary Network", "Healthy"}
	if subnetName != "" {
		header = append(header, "Subnet "+subnetName)
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	for i, cloudID := range cloudIDs {
		boostrappedStatus := ""
		healthyStatus := ""
		nodeIDStr := ""
		avagoVersion := ""
		roles := clusterConf.GetHostRoles(nodeConfigs[i])
		if clusterConf.IsAvalancheGoHost(cloudID) {
			boostrappedStatus = logging.Green.Wrap("BOOTSTRAPPED")
			if slices.Contains(notBootstrappedHosts, cloudID) {
				boostrappedStatus = logging.Red.Wrap("NOT_BOOTSTRAPPED")
			}
			healthyStatus = logging.Green.Wrap("OK")
			if slices.Contains(unhealthyHosts, cloudID) {
				healthyStatus = logging.Red.Wrap("UNHEALTHY")
			}
			nodeIDStr = nodeIDs[i]
			avagoVersion = avagoVersions[cloudID]
		}
		row := []string{
			cloudID,
			logging.Green.Wrap(nodeIDStr),
			nodeConfigs[i].ElasticIP,
			clusterConf.Network.Kind.String(),
			strings.Join(roles, ","),
			avagoVersion,
			boostrappedStatus,
			healthyStatus,
		}
		if subnetName != "" {
			syncedStatus := ""
			if clusterConf.MonitoringInstance != cloudID {
				syncedStatus = logging.Red.Wrap("NOT_BOOTSTRAPPED")
				if slices.Contains(subnetSyncedHosts, cloudID) {
					syncedStatus = logging.Green.Wrap("SYNCED")
				}
				if slices.Contains(subnetValidatingHosts, cloudID) {
					syncedStatus = logging.Green.Wrap("VALIDATING")
				}
			}
			row = append(row, syncedStatus)
		}
		table.Append(row)
	}
	table.Render()
}

func removeColors(s string) string {
	bs, err := ansi.Strip([]byte(s))
	if err != nil {
		return s
	}
	return string(bs)
}

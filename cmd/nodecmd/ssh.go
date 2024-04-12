// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

var isParallel bool

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [clusterName|nodeID|instanceID|IP] [cmd]",
		Short: "(ALPHA Warning) Execute ssh command on node(s)",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node ssh command execute a given command [cmd] using ssh on all nodes in the cluster if ClusterName is given.
If no command is given, just prints the ssh command to be used to connect to each node in the cluster.
For provided NodeID or InstanceID or IP, the command [cmd] will be executed on that node.
If no [cmd] is provided for the node, it will open ssh shell there.
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(0),
		RunE:         sshNode,
	}
	cmd.Flags().BoolVar(&isParallel, "parallel", false, "run ssh command on all nodes in parallel")
	return cmd
}

func sshNode(_ *cobra.Command, args []string) error {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if len(clustersConfig.Clusters) == 0 {
		ux.Logger.PrintToUser("There are no clusters defined.")
		return nil
	}
	if len(args) == 0 {
		// provide ssh connection string for all clusters
		for clusterName, clusterConfig := range clustersConfig.Clusters {
			err := printClusterConnectionString(clusterName, clusterConfig.Network.Kind.String())
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		clusterNameOrNodeID := args[0]
		cmd := strings.Join(args[1:], " ")
		if err := checkCluster(clusterNameOrNodeID); err == nil {
			// clusterName detected
			if len(args[1:]) == 0 {
				return printClusterConnectionString(clusterNameOrNodeID, clustersConfig.Clusters[clusterNameOrNodeID].Network.Kind.String())
			} else {
				clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterNameOrNodeID))
				if err != nil {
					return err
				}
				monitoringInventoryPath := filepath.Join(app.GetAnsibleInventoryDirPath(clusterNameOrNodeID), constants.MonitoringDir)
				if utils.DirectoryExists(monitoringInventoryPath) {
					monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
					if err != nil {
						return err
					}
					clusterHosts = append(clusterHosts, monitoringHosts...)
				}
				loadTestInventoryPath := filepath.Join(app.GetAnsibleInventoryDirPath(clusterNameOrNodeID), constants.LoadTestDir)
				if utils.DirectoryExists(loadTestInventoryPath) {
					loadTestHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(loadTestInventoryPath)
					if err != nil {
						return err
					}
					clusterHosts = append(clusterHosts, loadTestHosts...)
				}
				return sshHosts(clusterHosts, cmd, clustersConfig.Clusters[clusterNameOrNodeID])
			}
		} else {
			// try to detect nodeID
			for clusterName := range clustersConfig.Clusters {
				clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
				if err != nil {
					return err
				}
				monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
				if utils.DirectoryExists(monitoringInventoryPath) {
					monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
					if err != nil {
						return err
					}
					clusterHosts = append(clusterHosts, monitoringHosts...)
				}
				selectedHost := utils.Filter(clusterHosts, func(h *models.Host) bool {
					_, cloudHostID, _ := models.HostAnsibleIDToCloudID(h.NodeID)
					hostNodeID, _ := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
					return h.GetCloudID() == clusterNameOrNodeID || hostNodeID.String() == clusterNameOrNodeID || h.IP == clusterNameOrNodeID
				})
				switch {
				case len(selectedHost) == 0:
					continue
				case len(selectedHost) > 2:
					return fmt.Errorf("more then 1 node found for %s", clusterNameOrNodeID)
				default:
					return sshHosts(selectedHost, cmd, clustersConfig.Clusters[clusterName])
				}
			}
		}
		return fmt.Errorf("cluster or node %s not found", clusterNameOrNodeID)
	}
}

func printNodeInfo(host *models.Host, clusterConf models.ClusterConfig, result string) error {
	nodeConfig, err := app.LoadClusterNodeConfig(host.GetCloudID())
	if err != nil {
		return err
	}
	nodeIDStr := "----------------------------------------"
	if clusterConf.IsAvalancheGoHost(host.GetCloudID()) {
		nodeID, err := getNodeID(app.GetNodeInstanceDirPath(host.GetCloudID()))
		if err != nil {
			return err
		}
		nodeIDStr = nodeID.String()
	}
	roles := clusterConf.GetHostRoles(nodeConfig)
	rolesStr := strings.Join(roles, ",")
	if rolesStr != "" {
		rolesStr = " [" + rolesStr + "]"
	}
	ux.Logger.PrintToUser("  [Node %s (%s) %s%s] %s", host.GetCloudID(), nodeIDStr, nodeConfig.ElasticIP, rolesStr, result)
	return nil
}

func sshHosts(hosts []*models.Host, cmd string, clusterConf models.ClusterConfig) error {
	if cmd != "" {
		// execute cmd
		wg := sync.WaitGroup{}
		nowExecutingMutex := sync.Mutex{}
		wgResults := models.NodeResults{}
		for _, host := range hosts {
			wg.Add(1)
			go func(nodeResults *models.NodeResults, host *models.Host) {
				if !isParallel {
					nowExecutingMutex.Lock()
					defer nowExecutingMutex.Unlock()
					if err := printNodeInfo(host, clusterConf, ""); err != nil {
						ux.Logger.RedXToUser("Error getting node %s info due to : %s", host.GetCloudID(), err)
					}
				}
				defer wg.Done()
				splitCmdLine := strings.Split(utils.GetSSHConnectionString(host.IP, host.SSHPrivateKeyPath), " ")
				splitCmdLine = append(splitCmdLine, cmd)
				cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...)
				cmd.Env = os.Environ()
				outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
				if !isParallel {
					_, _ = utils.SetupRealtimeCLIOutput(cmd, true, true)
				}
				if _, err := outBuf.ReadFrom(errBuf); err != nil {
					nodeResults.AddResult(host.NodeID, outBuf, err)
				}
				if err := cmd.Run(); err != nil {
					nodeResults.AddResult(host.NodeID, outBuf, err)
				} else {
					nodeResults.AddResult(host.NodeID, outBuf, nil)
				}
			}(&wgResults, host)
		}
		wg.Wait()
		if wgResults.HasErrors() {
			return fmt.Errorf("failed to ssh node(s) %s", wgResults.GetErrorHostMap())
		}
		if isParallel {
			for hostID, result := range wgResults.GetResultMap() {
				for _, host := range hosts {
					if host.GetCloudID() == hostID {
						if err := printNodeInfo(host, clusterConf, fmt.Sprintf("%v", result)); err != nil {
							ux.Logger.RedXToUser("Error getting node %s info due to : %s", host.GetCloudID(), err)
						}
					}
				}
			}
		}
	} else {
		// open shell
		switch {
		case len(hosts) > 1:
			return fmt.Errorf("cannot open ssh shell on multiple nodes: %s", strings.Join(utils.Map(hosts, func(h *models.Host) string { return h.GetCloudID() }), ", "))
		case len(hosts) == 0:
			return fmt.Errorf("no nodes found")
		default:
			selectedHost := hosts[0]
			splitCmdLine := strings.Split(utils.GetSSHConnectionString(selectedHost.IP, selectedHost.SSHPrivateKeyPath), " ")
			cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...)
			cmd.Env = os.Environ()
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				ux.Logger.PrintToUser("Error: %s", err)
				return err
			}
			ux.Logger.PrintToUser("[%s] shell closed to %s", selectedHost.GetCloudID(), selectedHost.IP)
		}
	}
	return nil
}

func printClusterConnectionString(clusterName string, networkName string) error {
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if clusterConf.External {
		ux.Logger.PrintToUser("Cluster: %s (%s) EXTERNAL", logging.LightBlue.Wrap(clusterName), logging.Green.Wrap(networkName))
	} else {
		ux.Logger.PrintToUser("Cluster: %s (%s)", logging.LightBlue.Wrap(clusterName), logging.Green.Wrap(networkName))
	}
	clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
	if utils.DirectoryExists(monitoringInventoryPath) {
		monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
		if err != nil {
			return err
		}
		clusterHosts = append(clusterHosts, monitoringHosts...)
	}
	for _, host := range clusterHosts {
		ux.Logger.PrintToUser(utils.GetSSHConnectionString(host.IP, host.SSHPrivateKeyPath))
	}
	ux.Logger.PrintToUser("")
	return nil
}

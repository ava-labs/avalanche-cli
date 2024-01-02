// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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
			return printClusterConnectionString(clusterName, clusterConfig.Network.Name())
		}
		return nil
	} else {
		clusterNameOrNodeID := args[0]
		cmd := strings.Join(args[1:], " ")
		if err := checkCluster(clusterNameOrNodeID); err == nil {
			// clusterName detected
			if len(args[1:]) == 0 {
				return printClusterConnectionString(clusterNameOrNodeID, clustersConfig.Clusters[clusterNameOrNodeID].Network.Name())
			} else {
				clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterNameOrNodeID))
				if err != nil {
					return err
				}
				return sshHosts(clusterHosts, cmd)
			}
		} else {
			// try to detect nodeID
			for clusterName := range clustersConfig.Clusters {
				clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
				if err != nil {
					return err
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
					return sshHosts(selectedHost, cmd)
				}
			}
		}
		return fmt.Errorf("cluster or node %s not found", clusterNameOrNodeID)
	}
}

func sshHosts(hosts []*models.Host, cmd string) error {
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
				}
				defer wg.Done()
				splitCmdLine := strings.Split(utils.GetSSHConnectionString(host.IP, host.SSHPrivateKeyPath), " ")
				splitCmdLine = append(splitCmdLine, cmd)
				cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...) //nolint: gosec
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
				ux.Logger.PrintToUser("[%s] %s", hostID, fmt.Sprintf("%v", result))
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
			cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...) //nolint: gosec
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
	ux.Logger.PrintToUser("Cluster %q (%s)", clusterName, networkName)
	clusterHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	for _, host := range clusterHosts {
		ux.Logger.PrintToUser(utils.GetSSHConnectionString(host.IP, host.SSHPrivateKeyPath))
	}
	ux.Logger.PrintToUser("")
	return nil
}

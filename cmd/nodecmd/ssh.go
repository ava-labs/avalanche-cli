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
		Use:   "ssh [clusterName] [cmd]",
		Short: "(ALPHA Warning) Execute ssh command on node(s)",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node ssh command execute a given command using ssh on all nodes in the cluster.
If no command is given, just prints the ssh cmdLine to be used to connect to each node.
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(0),
		RunE:         sshNode,
	}
	cmd.Flags().BoolVar(&isParallel, "parallel", true, "run ssh command on all nodes in parallel")
	return cmd
}

func sshNode(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
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
		for clusterName, clusterConfig := range clustersConfig.Clusters {
			ux.Logger.PrintToUser("Cluster %q (%s)", clusterName, clusterConfig.Network.Name())
			if err := sshCluster([]string{clusterName}, "  "); err != nil {
				return err
			}
			ux.Logger.PrintToUser("")
		}
		return nil
	}
	return sshCluster(args, "")
}

func sshCluster(args []string, indent string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	nowExecutingMutex := sync.Mutex{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		cmdLine := fmt.Sprintf("%s %s", utils.GetSSHConnectionString(host.IP, host.SSHPrivateKeyPath), strings.Join(args[1:], " "))
		ux.Logger.PrintToUser("%s[%s] RUN: %s", indent, host.GetCloudID(), cmdLine)
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			if !isParallel {
				nowExecutingMutex.Lock()
				defer nowExecutingMutex.Unlock()
			}
			defer wg.Done()
			if len(args) > 1 {
				splitCmdLine := strings.Split(cmdLine, " ")
				cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...) //nolint: gosec
				cmd.Env = os.Environ()
				outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
				if !isParallel {
					_, _ = utils.SetupRealtimeCLIOutput(cmd, true, true)
				}
				if _, err := outBuf.ReadFrom(errBuf); err != nil {
					nodeResults.AddResult(host.NodeID, outBuf, err)
				}
				err = cmd.Run()
				if err != nil {
					nodeResults.AddResult(host.NodeID, outBuf, err)
				}
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
			ux.Logger.PrintToUser("%s[%s] %s", indent, hostID, fmt.Sprintf("%v", result))
		}
	}
	return nil
}

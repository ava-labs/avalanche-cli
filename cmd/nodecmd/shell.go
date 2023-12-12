// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSHellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell [clusterName] [nodeID]",
		Short: "(ALPHA Warning) Start ssh shell on node",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node shell command starts remote shell using ssh on specific node in the cluster.
NodeID InstanceID or IP can be used
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		RunE:         shellNode,
	}

	return cmd
}

func shellNode(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("invalid number of arguments")
	}
	clusterName := args[0]
	nodeID := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	selectedHost := utils.Filter(allHosts, func(h *models.Host) bool {
		_, cloudHostID, _ := models.HostAnsibleIDToCloudID(h.NodeID)
		hostNodeID, _ := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
		return h.GetCloudID() == nodeID || hostNodeID.String() == nodeID || h.IP == nodeID
	})
	if len(selectedHost) == 0 {
		return fmt.Errorf("node %s not found in cluster %s", nodeID, clusterName)
	} else {
		splitCmdLine := strings.Split(utils.GetSSHConnectionString(selectedHost[0].IP, selectedHost[0].SSHPrivateKeyPath), " ")
		cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...) //nolint: gosec
		cmd.Env = os.Environ()
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			ux.Logger.PrintToUser("Error: %s", err)
			return err
		}
	}
	ux.Logger.PrintToUser("%s[%s] shell closed to %s", clusterName, selectedHost[0].GetCloudID(), selectedHost[0].IP)
	return nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newWhitelistSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist-ssh <clusterName> \"<sshPubKey>\"",
		Short: "(ALPHA Warning) Whitelist SSH public key for SSH access to all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node whitelist-ssh command adds SSH public key to all nodes in the cluster `,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		RunE:         whitelistSSH,
	}
	return cmd
}

func whitelistSSH(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	sshPubKey := strings.Trim(args[1], "\"'")
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if !utils.IsSSHPubKey(sshPubKey) {
		return fmt.Errorf("invalid SSH public key: %s", sshPubKey)
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := ssh.RunSSHWhitelistPubKey(host, sshPubKey); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			ux.Logger.GreenCheckmarkToUser(utils.ScriptLog(host.NodeID, "Whitelisted SSH public key"))
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		ux.Logger.RedXToUser("Failed to whitelist SSH public key for node(s) %s", wgResults.GetErrorHostMap())
		return fmt.Errorf("failed to whitelist SSH public key for node(s) %s", wgResults.GetErrorHostMap())
	}
	return nil
}

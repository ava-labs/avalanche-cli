// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/prompts"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	"github.com/ava-labs/avalanche-cli/pkg/docker"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

var (
	nodeIPs          []string
	sshKeyPaths      []string
	overrideExisting bool
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Sets up a new Avalanche Node on remote server",
		Long: `The node setup command installs Avalanche Go on specified remote servers. 
To run the command, the remote servers' IP addresses and SSH private keys are required. 

Currently, only Ubuntu operating system is supported.`,
		Args:              cobrautils.ExactArgs(0),
		RunE:              setupNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.NonLocalSupportedNetworkOptions)
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&useAvalanchegoVersionFromSubnet, "avalanchego-version-from-subnet", "", "install latest avalanchego version, that is compatible with the given subnet, on node/s")
	cmd.Flags().BoolVar(&publicHTTPPortAccess, "public-http-port", false, "allow public access to avalanchego HTTP port")
	cmd.Flags().StringArrayVar(&nodeIPs, "node-ips", []string{}, "IP addresses of nodes")
	cmd.Flags().StringArrayVar(&sshKeyPaths, "ssh-key-paths", []string{}, "ssh key paths")
	cmd.Flags().BoolVar(&useSSHAgent, "use-ssh-agent", false, "use ssh agent(ex: Yubikey) for ssh auth")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	cmd.Flags().BoolVar(&partialSync, "partial-sync", true, "primary network partial sync")
	cmd.Flags().BoolVar(&overrideExisting, "override-existing", true, "override existing staking files")
	return cmd
}

func setup(hosts []*models.Host, avalancheGoVersion string, network models.Network) error {
	if globalNetworkFlags.UseDevnet {
		partialSync = false
		ux.Logger.PrintToUser("disabling partial sync default for devnet")
	}
	ux.Logger.PrintToUser("Setting up Avalanche node(s)...")
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	spinSession := ux.NewUserSpinner()

	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := host.Connect(0); err != nil {
				nodeResults.AddResult(host.IP, nil, err)
				return
			}
			if err := provideStakingCertAndKey(host); err != nil {
				nodeResults.AddResult(host.IP, nil, err)
				return
			}
			spinner := spinSession.SpinToUser(utils.ScriptLog(host.IP, "Setup Node"))
			if err := ssh.RunSSHSetupNode(host, app.Conf.GetConfigPath()); err != nil {
				nodeResults.AddResult(host.IP, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			if err := ssh.RunSSHSetupDockerService(host); err != nil {
				nodeResults.AddResult(host.IP, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			ux.SpinComplete(spinner)
			spinner = spinSession.SpinToUser(utils.ScriptLog(host.IP, "Setup AvalancheGo"))
			// check if host is a API host
			if err := docker.ComposeSSHSetupNode(host,
				network,
				avalancheGoVersion,
				bootstrapIDs,
				bootstrapIPs,
				partialSync,
				genesisPath,
				upgradePath,
				addMonitoring,
				host.APINode); err != nil {
				nodeResults.AddResult(host.IP, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			ux.SpinComplete(spinner)
		}(&wgResults, host)
	}
	wg.Wait()
	spinSession.Stop()
	for _, node := range hosts {
		if wgResults.HasIDWithError(node.NodeID) {
			ux.Logger.RedXToUser("Node %s has ERROR: %s", node.IP, wgResults.GetErrorHostMap()[node.IP])
		}
	}

	if wgResults.HasErrors() {
		return fmt.Errorf("failed to deploy node(s) %s", wgResults.GetErrorHostMap())
	} else {
		ux.Logger.PrintToUser(logging.Green.Wrap("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!"))
	}
	return nil
}

func setupNode(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		true,
		networkoptions.NonLocalSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	avaGoVersionSetting := node.AvalancheGoVersionSettings{
		UseAvalanchegoVersionFromSubnet:       useAvalanchegoVersionFromSubnet,
		UseLatestAvalanchegoReleaseVersion:    useLatestAvalanchegoReleaseVersion,
		UseLatestAvalanchegoPreReleaseVersion: useLatestAvalanchegoPreReleaseVersion,
		UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
	}
	avalancheGoVersion, err := node.GetAvalancheGoVersion(app, avaGoVersionSetting)
	if err != nil {
		return err
	}

	if !useSSHAgent {
		if len(nodeIPs) != len(sshKeyPaths) {
			return fmt.Errorf("--node-ips and --ssh-key-paths should have same number of values")
		}
	}

	if err = promptSetupNodes(); err != nil {
		return err
	}

	hosts := []*models.Host{}
	for i, nodeIP := range nodeIPs {
		sshKeyPath := ""
		if !useSSHAgent {
			sshKeyPath = sshKeyPaths[i]
		}
		hosts = append(hosts, &models.Host{
			SSHUser:           constants.RemoteSSHUser,
			IP:                nodeIP,
			SSHPrivateKeyPath: sshKeyPath,
		})
	}
	if err = setup(hosts, avalancheGoVersion, network); err != nil {
		return err
	}
	printSetupResults(hosts)
	return nil
}

func printSetupResults(hosts []*models.Host) {
	for _, host := range hosts {
		nodePath := app.GetNodeStakingDir(host.IP)
		certBytes, err := os.ReadFile(filepath.Join(nodePath, constants.StakerCertFileName))
		if err != nil {
			continue
		}
		nodeID, err := utils.ToNodeID(certBytes)
		if err != nil {
			continue
		}
		ux.Logger.PrintToUser("%s Public IP: %s | %s ", logging.Green.Wrap(">"), host.IP, logging.Green.Wrap(nodeID.String()))
		ux.Logger.PrintToUser("staker.crt, staker.key and signer.key are stored at %s. Please keep them safe, as these files can be used to fully recreate your node.", nodePath)
		ux.Logger.PrintLineSeparator()
	}
}

func promptSetupNodes() error {
	var err error
	var numNodes int
	ux.Logger.PrintToUser("Only Ubuntu operating system is supported")
	if len(nodeIPs) == 0 && len(sshKeyPaths) == 0 {
		numNodes, err = app.Prompt.CaptureInt(
			"How many Avalanche nodes do you want to setup?",
			prompts.ValidatePositiveInt,
		)
	}
	if err != nil {
		return err
	}
	for len(nodeIPs) < numNodes {
		ux.Logger.PrintToUser("Getting info for node %d", len(nodeIPs)+1)
		ipAddress, err := app.Prompt.CaptureString("What is the IP address of the node to be set up?")
		if err != nil {
			return err
		}
		nodeIPs = append(nodeIPs, ipAddress)
		ux.Logger.GreenCheckmarkToUser("Node %d:", len(nodeIPs))
		ux.Logger.PrintToUser("- IP Address: %s", ipAddress)
		if !useSSHAgent {
			sshKeyPath, err := app.Prompt.CaptureString("What is the key path of the private key that can be used to ssh into this node?")
			if err != nil {
				return err
			}
			sshKeyPaths = append(sshKeyPaths, sshKeyPath)
			ux.Logger.PrintToUser("- SSH Key Path: %s", sshKeyPath)
		}
	}
	return nil
}

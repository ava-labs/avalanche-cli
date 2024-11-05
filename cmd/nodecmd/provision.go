// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"sync"
	"time"

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
	"golang.org/x/exp/slices"
)

func newProvisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision [clusterName]",
		Short: "(ALPHA Warning) Create a new validator on cloud server",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node create command sets up a validator on a cloud server of your choice. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status 

The created node will be part of group of validators called <clusterName> 
and users can call node commands with <clusterName> so that the command
will apply to all nodes in the cluster`,
		Args:              cobrautils.ExactArgs(0),
		RunE:              provisionNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, createSupportedNetworkOptions)
	cmd.Flags().StringVar(&nodeType, "node-type", "", "cloud instance type. Use 'default' to use recommended default instance type")
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&useAvalanchegoVersionFromSubnet, "avalanchego-version-from-subnet", "", "install latest avalanchego version, that is compatible with the given subnet, on node/s")
	cmd.Flags().BoolVar(&publicHTTPPortAccess, "public-http-port", false, "allow public access to avalanchego HTTP port")
	cmd.Flags().StringArrayVar(&bootstrapIDs, "bootstrap-ids", []string{}, "nodeIDs of bootstrap nodes")
	cmd.Flags().StringArrayVar(&bootstrapIPs, "bootstrap-ips", []string{}, "IP:port pairs of bootstrap nodes")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	return cmd
}

func provisionNode(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		true,
		createSupportedNetworkOptions,
		"",
	)
	if network.Kind == models.EtnaDevnet {
		publicHTTPPortAccess = true // public http port access for etna devnet api for PoAManagerDeployment
		bootstrapIDs = constants.EtnaDevnetBootstrapNodeIDs
		bootstrapIPs = constants.EtnaDevnetBootstrapIPs

		// create genesis and upgrade files
		genesisTmpFile, err := os.CreateTemp("", "genesis")
		if err != nil {
			return err
		}
		if _, err := genesisTmpFile.Write(constants.EtnaDevnetGenesisData); err != nil {
			return err
		}
		if err := genesisTmpFile.Close(); err != nil {
			return err
		}
		genesisPath = genesisTmpFile.Name()

		upgradeTmpFile, err := os.CreateTemp("", "upgrade")
		if err != nil {
			return err
		}
		if _, err := upgradeTmpFile.Write(constants.EtnaDevnetUpgradeData); err != nil {
			return err
		}
		if err := upgradeTmpFile.Close(); err != nil {
			return err
		}
		upgradePath = upgradeTmpFile.Name()

		defer func() {
			_ = os.Remove(genesisTmpFile.Name())
			_ = os.Remove(upgradeTmpFile.Name())
		}()
	}
	network = models.NewNetworkFromCluster(network, clusterName)
	globalNetworkFlags.UseDevnet = network.Kind == models.Devnet // set globalNetworkFlags.UseDevnet to true if network is devnet for further use
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

	//inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	//allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(inventoryPath)

	//if err != nil {
	//	return err
	//}
	//hosts := utils.Filter(allHosts, func(h *models.Host) bool { return slices.Contains(cloudConfigMap.GetAllInstanceIDs(), h.GetCloudID()) })

	ux.Logger.PrintToUser("Starting bootstrap process on the newly created Avalanche node(s)...")
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	spinSession := ux.NewUserSpinner()

	startTime := time.Now()
	host := &models.Host{
		IP:                "",
		SSHPrivateKeyPath: "",
	}
	//for _, host := range hosts {
	wg.Add(1)
	go func(nodeResults *models.NodeResults, host *models.Host) {
		defer wg.Done()
		//if err := host.Connect(0); err != nil {
		//	nodeResults.AddResult(host.NodeID, nil, err)
		//	return
		//}
		if err := provideStakingCertAndKey(host); err != nil {
			nodeResults.AddResult(host.NodeID, nil, err)
			return
		}
		spinner := spinSession.SpinToUser(utils.ScriptLog(host.NodeID, "Setup Node"))
		if err := ssh.RunSSHSetupNode(host, app.Conf.GetConfigPath()); err != nil {
			nodeResults.AddResult(host.NodeID, nil, err)
			ux.SpinFailWithError(spinner, "", err)
			return
		}
		if err := ssh.RunSSHSetupDockerService(host); err != nil {
			nodeResults.AddResult(host.NodeID, nil, err)
			ux.SpinFailWithError(spinner, "", err)
			return
		}
		ux.SpinComplete(spinner)

		spinner = spinSession.SpinToUser(utils.ScriptLog(host.NodeID, "Setup AvalancheGo"))
		// check if host is a API host
		publicAccessToHTTPPort := slices.Contains(cloudConfigMap.GetAllAPIInstanceIDs(), host.GetCloudID()) || publicHTTPPortAccess
		if err := docker.ComposeSSHSetupNode(host,
			network,
			avalancheGoVersion,
			bootstrapIDs,
			bootstrapIPs,
			genesisPath,
			upgradePath,
			addMonitoring,
			publicAccessToHTTPPort); err != nil {
			nodeResults.AddResult(host.NodeID, nil, err)
			ux.SpinFailWithError(spinner, "", err)
			return
		}
		ux.SpinComplete(spinner)
	}(&wgResults, host)
	//}
	wg.Wait()
	ux.Logger.Info("Create and setup nodes time took: %s", time.Since(startTime))
	spinSession.Stop()
	if network.Kind == models.Devnet {
		if err := setupDevnet(clusterName, hosts, apiNodeIPMap); err != nil {
			return err
		}
	}
	for _, node := range hosts {
		if wgResults.HasNodeIDWithError(node.NodeID) {
			ux.Logger.RedXToUser("Node %s is ERROR with error: %s", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
		}
	}

	if wgResults.HasErrors() {
		return fmt.Errorf("failed to deploy node(s) %s", wgResults.GetErrorHostMap())
	} else {
		printResults(cloudConfigMap, publicIPMap, monitoringPublicIP)
		ux.Logger.PrintToUser(logging.Green.Wrap("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!"))
	}
	sendNodeCreateMetrics(cmd, cloudService, network.Name(), numNodesMetricsMap)
	return nil
}

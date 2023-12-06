// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Subnet. 
You can check the subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         syncSubnet,
	}

	return cmd
}

func getNodesWoEIPInAnsibleInventory(clusterNodes []string) []models.NodeConfig {
	nodesWoEIP := []models.NodeConfig{}
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			continue
		}
		if nodeConfig.ElasticIP == "" {
			nodesWoEIP = append(nodesWoEIP, nodeConfig)
		}
	}
	return nodesWoEIP
}

func getPublicIPForNodesWoEIP(nodesWoEIP []models.NodeConfig) (map[string]string, error) {
	lastRegion := ""
	var ec2Svc *awsAPI.AwsCloud
	var err error
	publicIPMap := make(map[string]string)
	var gcpCloud *gcpAPI.GcpCloud
	ux.Logger.PrintToUser("Getting Public IPs for nodes without static IPs ...")
	for _, node := range nodesWoEIP {
		if lastRegion == "" || node.Region != lastRegion {
			if node.CloudService == "" || node.CloudService == constants.AWSCloudService {
				ec2Svc, err = awsAPI.NewAwsCloud(awsProfile, node.Region)
				if err != nil {
					return nil, err
				}
			}
			lastRegion = node.Region
		}
		var publicIP map[string]string
		var err error
		if node.CloudService == constants.GCPCloudService {
			if gcpCloud == nil {
				gcpClient, projectName, _, err := getGCPCloudCredentials()
				if err != nil {
					return nil, err
				}
				gcpCloud, err = gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
				if err != nil {
					return nil, err
				}
			}
			publicIP, err = gcpCloud.GetInstancePublicIPs(node.Region, []string{node.NodeID})
			if err != nil {
				return nil, err
			}
		} else {
			publicIP, err = ec2Svc.GetInstancePublicIPs([]string{node.NodeID})
			if err != nil {
				return nil, err
			}
		}
		publicIPMap[node.NodeID] = publicIP[node.NodeID]
	}
	return publicIPMap, nil
}

func updateAnsiblePublicIPs(clusterName string) error {
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	nodesWoEIP := getNodesWoEIPInAnsibleInventory(clusterNodes)
	if len(nodesWoEIP) > 0 {
		publicIP, err := getPublicIPForNodesWoEIP(nodesWoEIP)
		if err != nil {
			return err
		}
		err = ansible.UpdateInventoryHostPublicIP(app.GetAnsibleInventoryDirPath(clusterName), publicIP)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer disconnectHosts(hosts)
	notBootstrappedNodes, err := checkHostsAreBootstrapped(hosts)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	notHealthyNodes, err := checkHostsAreHealthy(hosts)
	if err != nil {
		return err
	}
	if len(notHealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy, please fix the issue and again", notHealthyNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(hosts, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your VM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		switch sc.VM {
		case models.SubnetEvm:
			ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		case models.CustomVM:
			ux.Logger.PrintToUser("To modify your Custom VM binary: avalanche subnet upgrade vm %s --config", subnetName)
		}
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	network := clustersConfig.Clusters[clusterName].Network
	untrackedNodes, err := trackSubnet(hosts, subnetName, network)
	if err != nil {
		return err
	}
	if len(untrackedNodes) > 0 {
		return fmt.Errorf("node(s) %s failed to sync with subnet %s", untrackedNodes, subnetName)
	}
	ux.Logger.PrintToUser("Node(s) successfully started syncing with Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet syncing status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(
	hosts []*models.Host,
	subnetName string,
	network models.Network,
) ([]string, error) {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	networkFlag := ""
	switch network.Kind {
	case models.Local:
		networkFlag = "--local"
	case models.Devnet:
		networkFlag = "--devnet"
	case models.Fuji:
		networkFlag = "--fuji"
	case models.Mainnet:
		networkFlag = "--mainnet"
	}
	if err := subnetcmd.CallExportSubnet(subnetName, subnetPath); err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			subnetExportPath := filepath.Join("/tmp", filepath.Base(subnetPath))
			if err := ssh.RunSSHExportSubnet(host, subnetPath, subnetExportPath); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if err := ssh.RunSSHTrackSubnet(host, subnetName, subnetExportPath, networkFlag); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		fmt.Println(wgResults.GetErrorHostMap())
		return nil, fmt.Errorf("failed to track subnet for node(s) %s", wgResults.GetErrorHostMap())
	}
	return wgResults.GetErrorHosts(), nil
}

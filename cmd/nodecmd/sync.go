// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/gcp"
	"github.com/aws/aws-sdk-go/service/ec2"
	"google.golang.org/api/compute/v1"

	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"golang.org/x/exp/slices"

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
	var ec2Svc *ec2.EC2
	publicIPMap := make(map[string]string)
	var gcpClient *compute.Service
	var gcpProjectName string
	ux.Logger.PrintToUser("Getting Public IPs for nodes without static IPs ...")
	for _, node := range nodesWoEIP {
		if lastRegion == "" || node.Region != lastRegion {
			if node.CloudService == "" || node.CloudService == constants.AWSCloudService {
				// check for empty because we didn't set this value when it was only on AWS
				sess, err := getAWSCloudCredentials(awsProfile, node.Region, constants.GetAWSNodeIP, true)
				if err != nil {
					return nil, err
				}
				ec2Svc = ec2.New(sess)
			}
			lastRegion = node.Region
		}
		var publicIP map[string]string
		var err error
		if node.CloudService == constants.GCPCloudService {
			if gcpClient == nil {
				gcpClient, gcpProjectName, _, err = getGCPCloudCredentials()
				if err != nil {
					return nil, err
				}
			}
			publicIP, err = gcpAPI.GetInstancePublicIPs(gcpClient, gcpProjectName, node.Region, []string{node.NodeID})
			if err != nil {
				return nil, err
			}
		} else {
			publicIP, err = awsAPI.GetInstancePublicIPs(ec2Svc, []string{node.NodeID})
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
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(clusterName, subnetName)
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
	if err := setupBuildEnv(app.GetAnsibleInventoryDirPath(clusterName), ""); err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	network := clustersConfig.Clusters[clusterName].Network
	untrackedNodes, err := trackSubnet(clusterName, subnetName, network)
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

func parseAvalancheGoOutput(fileName string) (string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	if err = json.Unmarshal(byteValue, &result); err != nil {
		return "", err
	}
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		vmVersions, ok := nodeIDInterface["vmVersions"].(map[string]interface{})
		if ok {
			avalancheGoVersion, ok := vmVersions["platform"].(string)
			if ok {
				return avalancheGoVersion, nil
			}
		}
	}
	return "", nil
}

func checkForCompatibleAvagoVersion(configuredRPCVersion int) ([]string, error) {
	compatibleAvagoVersions, err := vm.GetAvailableAvalancheGoVersions(
		app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return nil, err
	}
	return compatibleAvagoVersions, nil
}

func checkAvalancheGoVersionCompatible(clusterName, subnetName string) ([]string, error) {
	if err := app.CreateAnsibleDir(); err != nil {
		return nil, err
	}
	ansibleNodeIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Checking compatibility of avalanche go version in cluster %s with Subnet EVM RPC of subnet %s ...", clusterName, subnetName))
	compatibleVersions := []string{}
	incompatibleNodes := []string{}
	if err := app.CreateAnsibleStatusDir(); err != nil {
		return nil, err
	}
	if err := ansible.RunAnsiblePlaybookCheckAvalancheGoVersion(app.GetAnsibleDir(), app.GetAvalancheGoJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), "all"); err != nil {
		return nil, err
	}
	for _, host := range ansibleNodeIDs {
		avalancheGoVersion, err := parseAvalancheGoOutput(app.GetAvalancheGoJSONFile() + "." + host)
		if err != nil {
			return nil, err
		}
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return nil, err
		}
		compatibleVersions, err = checkForCompatibleAvagoVersion(sc.RPCVersion)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(compatibleVersions, avalancheGoVersion) {
			incompatibleNodes = append(incompatibleNodes, host)
		}
	}
	if err := app.RemoveAnsibleStatusDir(); err != nil {
		return nil, err
	}
	if len(incompatibleNodes) > 0 {
		ux.Logger.PrintToUser(fmt.Sprintf("Compatible Avalanche Go versions are %s", strings.Join(compatibleVersions, ", ")))
	}
	return incompatibleNodes, nil
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(clusterName, subnetName string, network models.Network) ([]string, error) {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetName, subnetPath, network); err != nil {
		return nil, err
	}
	if err := ansible.RunAnsiblePlaybookSetupCLIFromSource(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), constants.SetupCLIFromSourceBranch, "all"); err != nil {
		return nil, err
	}
	if err := ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), subnetPath, "all"); err != nil {
		return nil, err
	}
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	untrackedNodes := []string{}
	for _, host := range hostAliases {
		// runs avalanche join subnet command
		if err = ansible.RunAnsiblePlaybookTrackSubnet(app.GetAnsibleDir(), network, subnetName, subnetPath, app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			untrackedNodes = append(untrackedNodes, host)
		}
	}
	return untrackedNodes, nil
}

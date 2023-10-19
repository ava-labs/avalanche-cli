// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/aws/aws-sdk-go/service/ec2"
	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/vm"

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

func getNodesWoEIPInAnsibleInventory(clusterNodes []string) []string {
	nodesWoEIP := []string{}
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			continue
		}
		if nodeConfig.ElasticIP == "" {
			nodesWoEIP = append(nodesWoEIP, node)
		}
	}
	return nodesWoEIP
}

func getPublicIPForNodesWoEIP(nodesWoEIP []string) (map[string]string, error) {
	lastRegion := ""
	var ec2Svc *ec2.EC2
	publicIPMap := make(map[string]string)
	for _, node := range nodesWoEIP {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			return nil, err
		}
		if nodeConfig.Region != lastRegion {
			sess, err := getAWSCloudCredentials(nodeConfig.Region, constants.GetAWSNodeIP)
			if err != nil {
				return nil, err
			}
			ec2Svc = ec2.New(sess)
			lastRegion = nodeConfig.Region
		}
		publicIP, err := awsAPI.GetInstancePublicIPs(ec2Svc, []string{node})
		if err != nil {
			return nil, err
		}
		publicIPMap[node] = publicIP[node]
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
	if err := setupBuildEnv(clusterName); err != nil {
		return err
	}
	untrackedNodes, err := trackSubnet(clusterName, subnetName, models.Fuji)
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

func parseAvalancheGoOutput(byteValue []byte) (string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
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
	ux.Logger.PrintToUser(fmt.Sprintf("Checking compatibility of avalanche go version in cluster %s with Subnet EVM RPC of subnet %s ...", clusterName, subnetName))
	compatibleVersions := []string{}
	incompatibleNodes := []string{}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	nodeResultChannel := make(chan models.NodeStringResult, len(hosts))
	parallelWaitGroup := sync.WaitGroup{}
	for _, host := range hosts {
		parallelWaitGroup.Add(1)
		go func(nodeResultChannel chan models.NodeStringResult, host models.Host) {
			defer parallelWaitGroup.Done()
			resp, err := ssh.RunSSHCheckAvalancheGoVersion(host)
			if err != nil {
				nodeResultChannel <- models.NodeStringResult{NodeID: host.NodeID, Value: constants.AvalancheGoVersionUnknown, Err: err}
			}
			avalancheGoVersion, err := parseAvalancheGoOutput(resp)
			if err != nil {
				nodeResultChannel <- models.NodeStringResult{NodeID: host.NodeID, Value: constants.AvalancheGoVersionUnknown, Err: err}
			}
			nodeResultChannel <- models.NodeStringResult{NodeID: host.NodeID, Value: avalancheGoVersion, Err: nil}
		}(nodeResultChannel, host)
	}
	parallelWaitGroup.Wait()
	close(nodeResultChannel)
	for avalancheGoVersion := range nodeResultChannel {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return nil, err
		}
		compatibleVersions, err = checkForCompatibleAvagoVersion(sc.RPCVersion)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(compatibleVersions, avalancheGoVersion.Value) {
			incompatibleNodes = append(incompatibleNodes, avalancheGoVersion.NodeID)
		}
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
	untrackedNodes := []string{}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	nodeResultChannel := make(chan models.NodeErrorResult, len(hosts))
	parallelWaitGroup := sync.WaitGroup{}
	for _, host := range hosts {
		parallelWaitGroup.Add(1)
		go func(nodeResultChannel chan models.NodeErrorResult, host models.Host) {
			defer parallelWaitGroup.Done()
			if err := ssh.RunSSHExportSubnet(host, subnetPath, "/tmp"); err != nil {
				nodeResultChannel <- models.NodeErrorResult{NodeID: host.NodeID, Err: err}
			}
			if err := ssh.RunSSHTrackSubnet(host, subnetName, subnetPath); err != nil {
				nodeResultChannel <- models.NodeErrorResult{NodeID: host.NodeID, Err: err}
			}
		}(nodeResultChannel, host)
	}
	parallelWaitGroup.Wait()
	close(nodeResultChannel)
	for nodeErr := range nodeResultChannel {
		if nodeErr.Err != nil {
			untrackedNodes = append(untrackedNodes, nodeErr.NodeID)
		}
	}
	return untrackedNodes, nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"golang.org/x/exp/maps"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var loadTestsToStop []string

func newLoadTestStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [clusterName]",
		Short: "(ALPHA Warning) Stops load test for an existing devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest stop command stops load testing for an existing devnet cluster and terminates the 
separate cloud server created to host the load test.`,

		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         stopLoadTest,
	}
	cmd.Flags().StringSliceVar(&loadTestsToStop, "load-test", []string{}, "stop specified load test node(s). Use comma to separate multiple load test instance names")
	return cmd
}

func getLoadTestInstancesInCluster(clusterName string) ([]string, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return nil, err
		}
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		return nil, fmt.Errorf("cluster %s doesn't exist", clusterName)
	}
	if clustersConfig.Clusters[clusterName].LoadTestInstance != nil {
		return maps.Keys(clustersConfig.Clusters[clusterName].LoadTestInstance), nil
	}
	return nil, fmt.Errorf("no load test instances found")
}

func checkLoadTestExists(clusterName, loadTestName string) (bool, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return false, err
		}
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		return false, fmt.Errorf("cluster %s doesn't exist", clusterName)
	}
	if clustersConfig.Clusters[clusterName].LoadTestInstance != nil {
		_, ok := clustersConfig.Clusters[clusterName].LoadTestInstance[loadTestName]
		return ok, nil
	}
	return false, nil
}

func stopLoadTest(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	var err error
	if len(loadTestsToStop) == 0 {
		loadTestsToStop, err = getLoadTestInstancesInCluster(clusterName)
		if err != nil {
			return err
		}
	}
	separateHostInventoryPath := app.GetLoadTestInventoryDir(clusterName)
	separateHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(separateHostInventoryPath)
	if err != nil {
		return err
	}
	removedLoadTestHosts := []*models.Host{}
	if len(loadTestsToStop) == 0 {
		return fmt.Errorf("no load test instances to stop in cluster %s", clusterName)
	}
	existingLoadTestInstance, err := getExistingLoadTestInstance(clusterName, loadTestsToStop[0])
	if err != nil {
		return err
	}
	nodeToStopConfig, err := app.LoadClusterNodeConfig(existingLoadTestInstance)
	if err != nil {
		return err
	}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	cloudSecurityGroupList, err := getCloudSecurityGroupList(clusterNodes)
	if err != nil {
		return err
	}
	filteredSGList := utils.Filter(cloudSecurityGroupList, func(sg regionSecurityGroup) bool { return sg.cloud == nodeToStopConfig.CloudService })
	if len(filteredSGList) == 0 {
		return fmt.Errorf("no endpoint found in the  %s", nodeToStopConfig.CloudService)
	}
	ec2SvcMap := make(map[string]*awsAPI.AwsCloud)
	for _, sg := range filteredSGList {
		sgEc2Svc, err := awsAPI.NewAwsCloud(awsProfile, sg.region)
		if err != nil {
			return err
		}
		ec2SvcMap[sg.region] = sgEc2Svc
	}
	for _, loadTestName := range loadTestsToStop {
		existingSeparateInstance, err = getExistingLoadTestInstance(clusterName, loadTestName)
		if err != nil {
			return err
		}
		if existingSeparateInstance == "" {
			return fmt.Errorf("no existing load test instance found in cluster %s", clusterName)
		}
		nodeConfig, err := app.LoadClusterNodeConfig(existingSeparateInstance)
		if err != nil {
			return err
		}
		hosts := utils.Filter(separateHosts, func(h *models.Host) bool { return h.GetCloudID() == nodeConfig.NodeID })
		if len(hosts) == 0 {
			return fmt.Errorf("host %s is not found in hosts inventory file", nodeConfig.NodeID)
		}
		host := hosts[0]
		loadTestResultFileName := fmt.Sprintf("loadtest_%s.txt", loadTestName)
		// Download the load test result from remote cloud server to local machine
		if err = ssh.RunSSHDownloadFile(host, fmt.Sprintf("/home/ubuntu/%s", loadTestResultFileName), filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), loadTestResultFileName)); err != nil {
			ux.Logger.RedXToUser("Unable to download load test result %s to local machine due to %s", loadTestResultFileName, err.Error())
		}
		switch nodeConfig.CloudService {
		case constants.AWSCloudService:
			loadTestNodeConfig, separateHostRegion, err := getNodeCloudConfig(existingSeparateInstance)
			if err != nil {
				return err
			}
			loadTestEc2SvcMap, err := getAWSMonitoringEC2Svc(awsProfile, separateHostRegion)
			if err != nil {
				return err
			}
			if err = destroyNode(existingSeparateInstance, clusterName, loadTestName, loadTestEc2SvcMap[separateHostRegion], nil); err != nil {
				return err
			}
			for _, sg := range filteredSGList {
				if err = deleteMonitoringSecurityGroupRule(ec2SvcMap[sg.region], loadTestNodeConfig.PublicIPs[0], sg.securityGroup, sg.region); err != nil {
					ux.Logger.RedXToUser("unable to delete IP address %s from security group %s in region %s due to %s, please delete it manually",
						loadTestNodeConfig.PublicIPs[0], sg.securityGroup, sg.region, err.Error())
				}
			}
		case constants.GCPCloudService:
			var gcpClient *gcpAPI.GcpCloud
			gcpClient, _, _, _, _, err = getGCPConfig(true)
			if err != nil {
				return err
			}
			if err = destroyNode(existingSeparateInstance, clusterName, loadTestName, nil, gcpClient); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
		}
		removedLoadTestHosts = append(removedLoadTestHosts, host)
	}
	return updateLoadTestInventory(separateHosts, removedLoadTestHosts, clusterName, separateHostInventoryPath)
}

func updateLoadTestInventory(separateHosts, removedLoadTestHosts []*models.Host, clusterName, separateHostInventoryPath string) error {
	var remainingLoadTestHosts []*models.Host
	for _, loadTestHost := range separateHosts {
		filteredHosts := utils.Filter(removedLoadTestHosts, func(h *models.Host) bool { return h.IP == loadTestHost.IP })
		if len(filteredHosts) == 0 {
			remainingLoadTestHosts = append(remainingLoadTestHosts, loadTestHost)
		}
	}
	if err := removeLoadTestInventoryDir(clusterName); err != nil {
		return err
	}
	if len(remainingLoadTestHosts) > 0 {
		for _, loadTestHost := range remainingLoadTestHosts {
			nodeConfig, err := app.LoadClusterNodeConfig(loadTestHost.GetCloudID())
			if err != nil {
				return err
			}
			if err = ansible.CreateAnsibleHostInventory(separateHostInventoryPath, loadTestHost.SSHPrivateKeyPath, nodeConfig.CloudService, map[string]string{nodeConfig.NodeID: nodeConfig.ElasticIP}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func destroyNode(node, clusterName, loadTestName string, ec2Svc *awsAPI.AwsCloud, gcpClient *gcpAPI.GcpCloud) error {
	nodeConfig, err := app.LoadClusterNodeConfig(node)
	if err != nil {
		ux.Logger.RedXToUser("Failed to destroy node %s", node)
		return err
	}
	if nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService {
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		if err = ec2Svc.DestroyAWSNode(nodeConfig, ""); err != nil {
			if isExpiredCredentialError(err) {
				ux.Logger.PrintToUser("")
				printExpiredCredentialsOutput(awsProfile)
				return nil
			}
			if !errors.Is(err, awsAPI.ErrNodeNotFoundToBeRunning) {
				return err
			}
			ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
		}
	} else {
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		if err = gcpClient.DestroyGCPNode(nodeConfig, ""); err != nil {
			if !errors.Is(err, gcpAPI.ErrNodeNotFoundToBeRunning) {
				return err
			}
			ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
		}
	}
	ux.Logger.GreenCheckmarkToUser("Node instance %s successfully destroyed!", nodeConfig.NodeID)
	if err := removeDeletedNodeDirectory(node); err != nil {
		ux.Logger.RedXToUser("Failed to delete node config for node %s due to %s", node, err.Error())
		return err
	}
	if err := removeLoadTestNodeFromClustersConfig(clusterName, loadTestName); err != nil {
		ux.Logger.RedXToUser("Failed to delete node config for node %s due to %s", node, err.Error())
		return err
	}
	return nil
}

func removeLoadTestNodeFromClustersConfig(clusterName, loadTestName string) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.Clusters != nil {
		if _, ok := clustersConfig.Clusters[clusterName]; !ok {
			return fmt.Errorf("cluster %s is not found in cluster config", clusterName)
		}
		clusterConfig := clustersConfig.Clusters[clusterName]
		if _, ok := clusterConfig.LoadTestInstance[loadTestName]; ok {
			if clustersConfig.Clusters != nil {
				delete(clusterConfig.LoadTestInstance, loadTestName)
			}
		}
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func removeLoadTestInventoryDir(clusterName string) error {
	return os.RemoveAll(app.GetLoadTestInventoryDir(clusterName))
}

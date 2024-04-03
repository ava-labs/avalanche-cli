// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"golang.org/x/exp/maps"
	"golang.org/x/net/context"

	"github.com/spf13/cobra"
)

var (
	authorizeRemove bool
	authorizeAll    bool
)

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [clusterName]",
		Short: "(ALPHA Warning) Destroys all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node destroy command terminates all running nodes in cloud server and deletes all storage disks.

If there is a static IP address attached, it will be released.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         destroyNodes,
	}
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to release cloud resources")
	cmd.Flags().BoolVar(&authorizeRemove, "authorize-remove", false, "authorize CLI to remove all local files related to cloud nodes")
	cmd.Flags().BoolVarP(&authorizeAll, "authorize-all", "y", false, "authorize all CLI requests")

	return cmd
}

func removeNodeFromClustersConfig(clusterName string) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.Clusters != nil {
		delete(clustersConfig.Clusters, clusterName)
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func removeDeletedNodeDirectory(clusterName string) error {
	return os.RemoveAll(app.GetNodeInstanceDirPath(clusterName))
}

func removeClusterInventoryDir(clusterName string) error {
	return os.RemoveAll(app.GetAnsibleInventoryDirPath(clusterName))
}

func getDeleteConfigConfirmation() error {
	if authorizeRemove {
		return nil
	}
	ux.Logger.PrintToUser("Please note that if your node(s) are validating a Subnet, destroying them could cause Subnet instability and it is irreversible")
	confirm := "Running this command will delete all stored files associated with your cloud server. Do you want to proceed? " +
		fmt.Sprintf("Stored files can be found at %s", app.GetNodesDir())
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("abort avalanche node destroy command")
	}
	return nil
}

func removeClustersConfigFiles(clusterName string) error {
	if err := removeClusterInventoryDir(clusterName); err != nil {
		return err
	}
	return removeNodeFromClustersConfig(clusterName)
}

func destroyNodes(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if authorizeAll {
		authorizeAccess = true
		authorizeRemove = true
	}
	if err := getDeleteConfigConfirmation(); err != nil {
		return err
	}
	nodesToStop, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	monitoringNode, err := getClusterMonitoringNode(clusterName)
	if err != nil {
		return err
	}
	if monitoringNode != "" {
		nodesToStop = append(nodesToStop, monitoringNode)
	}
	// stop all load test nodes if specified
	ltHosts, err := getLoadTestInstancesInCluster(clusterName)
	if err != nil {
		return err
	}
	for _, loadTestName := range ltHosts {
		ltInstance, err := getExistingLoadTestInstance(clusterName, loadTestName)
		if err != nil {
			return err
		}
		nodesToStop = append(nodesToStop, ltInstance)
	}
	awmRelayerHost, err := getAWMRelayerHost(clusterName)
	if err != nil {
		return err
	}
	nodeErrors := map[string]error{}
	lastRegion := ""
	var ec2Svc *awsAPI.AwsCloud
	var gcpCloud *gcpAPI.GcpCloud
	for _, node := range nodesToStop {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			nodeErrors[node] = err
			ux.Logger.PrintToUser("Failed to destroy node %s due to %s", node, err.Error())
			continue
		}
		if nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService {
			if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
				return fmt.Errorf("cloud access is required")
			}
			// need to check if it's empty because we didn't set cloud service when only using AWS
			if nodeConfig.Region != lastRegion {
				ec2Svc, err = awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
				if err != nil {
					return err
				}
				lastRegion = nodeConfig.Region
			}
			if err = ec2Svc.DestroyAWSNode(nodeConfig, clusterName); err != nil {
				if isExpiredCredentialError(err) {
					ux.Logger.PrintToUser("")
					printExpiredCredentialsOutput(awsProfile)
					return nil
				}
				if !errors.Is(err, awsAPI.ErrNodeNotFoundToBeRunning) {
					nodeErrors[node] = err
					continue
				}
				ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
			}
			if awmRelayerHost != nil {
				if err := deleteAWSAWMRelayerSecurityGroupRule(ec2Svc, &nodeConfig, awmRelayerHost); err != nil {
					ux.Logger.RedXToUser("unable to delete IP address %s from security group %s in region %s due to %s, please delete it manually",
						awmRelayerHost.IP, nodeConfig.SecurityGroup, nodeConfig.Region, err.Error())
				}
			}
			if err = deleteMonitoringSecurityGroupRule(ec2Svc, nodeConfig.ElasticIP, nodeConfig.SecurityGroup, nodeConfig.Region); err != nil {
				ux.Logger.RedXToUser("unable to delete IP address %s from security group %s in region %s due to %s, please delete it manually",
					nodeConfig.ElasticIP, nodeConfig.SecurityGroup, nodeConfig.Region, err.Error())
			}
		} else {
			if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
				return fmt.Errorf("cloud access is required")
			}
			if gcpCloud == nil {
				gcpClient, projectName, _, err := getGCPCloudCredentials()
				if err != nil {
					return err
				}
				gcpCloud, err = gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
				if err != nil {
					return err
				}
			}
			if err = gcpCloud.DestroyGCPNode(nodeConfig, clusterName); err != nil {
				if !errors.Is(err, gcpAPI.ErrNodeNotFoundToBeRunning) {
					nodeErrors[node] = err
					continue
				}
				ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
			}
		}
		ux.Logger.PrintToUser("Node instance %s in cluster %s successfully destroyed!", nodeConfig.NodeID, clusterName)
		if err := removeDeletedNodeDirectory(node); err != nil {
			ux.Logger.PrintToUser("Failed to delete node config for node %s due to %s", node, err.Error())
			return err
		}
	}
	if len(nodeErrors) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for node, nodeErr := range nodeErrors {
			if strings.Contains(nodeErr.Error(), constants.ErrReleasingGCPStaticIP) {
				ux.Logger.PrintToUser("Node is destroyed, but failed to release static ip address for node %s due to %s", node, nodeErr)
			} else {
				ux.Logger.PrintToUser("Failed to destroy node %s due to %s", node, nodeErr)
			}
		}
		return fmt.Errorf("failed to destroy node(s) %s", maps.Keys(nodeErrors))
	} else {
		ux.Logger.PrintToUser("All nodes in cluster %s are successfully destroyed!", clusterName)
	}

	return removeClustersConfigFiles(clusterName)
}

func getClusterMonitoringNode(clusterName string) (string, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return "", err
		}
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		return "", fmt.Errorf("cluster %q does not exist", clusterName)
	}
	return clustersConfig.Clusters[clusterName].MonitoringInstance, nil
}

func checkCluster(clusterName string) error {
	_, err := getClusterNodes(clusterName)
	return err
}

func getClusterNodes(clusterName string) ([]string, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return nil, err
		}
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		return nil, fmt.Errorf("cluster %q does not exist", clusterName)
	}
	clusterNodes := clustersConfig.Clusters[clusterName].Nodes
	if len(clusterNodes) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	return clusterNodes, nil
}

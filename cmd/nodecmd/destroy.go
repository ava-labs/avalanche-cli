// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	nodePkg "github.com/ava-labs/avalanche-cli/pkg/node"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"golang.org/x/exp/maps"
	"golang.org/x/net/context"

	"github.com/spf13/cobra"
)

var (
	authorizeRemove bool
	authorizeAll    bool
	destroyAll      bool
)

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [clusterName]",
		Short: "(ALPHA Warning) Destroys all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node destroy command terminates all running nodes in cloud server and deletes all storage disks.

If there is a static IP address attached, it will be released.`,
		Args: cobrautils.MinimumNArgs(0),
		RunE: destroyNodes,
	}
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to release cloud resources")
	cmd.Flags().BoolVar(&authorizeRemove, "authorize-remove", false, "authorize CLI to remove all local files related to cloud nodes")
	cmd.Flags().BoolVarP(&authorizeAll, "authorize-all", "y", false, "authorize all CLI requests")
	cmd.Flags().BoolVar(&destroyAll, "all", false, "destroy all existing clusters created by Avalanche CLI")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")

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

func CallDestroyNode(clusterName string) error {
	authorizeAll = true
	return destroyNodes(nil, []string{clusterName})
}

// We need to get which cloud service is being used on a cluster
// getFirstAvailableNode gets first node in the cluster that still has its node_config.json
// This is because some nodes might have had their node_config.json file deleted as part of
// deletion process but if an error occurs during deletion process, the node might still exist
// as part of the cluster in cluster_config.json
// If all nodes in the cluster no longer have their node_config.json files, getFirstAvailableNode
// will return false in its second return value
func getFirstAvailableNode(nodesToStop []string) (string, bool) {
	firstAvailableNode := nodesToStop[0]
	noAvailableNodesFound := false
	for index, node := range nodesToStop {
		nodeConfigPath := app.GetNodeConfigPath(node)
		if !utils.FileExists(nodeConfigPath) {
			if index == len(nodesToStop)-1 {
				noAvailableNodesFound = true
			}
			continue
		}
		firstAvailableNode = node
	}
	return firstAvailableNode, noAvailableNodesFound
}

func Cleanup() error {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	clusterNames := maps.Keys(clustersConfig.Clusters)
	for _, clusterName := range clusterNames {
		if err = CallDestroyNode(clusterName); err != nil {
			// we only return error for invalid cloud credentials
			// silence for other errors
			// TODO: differentiate between AWS and GCP credentials
			if strings.Contains(err.Error(), "invalid cloud credentials") {
				return fmt.Errorf("invalid AWS credentials")
			}
		}
	}
	ux.Logger.PrintToUser("all existing instances created by Avalanche CLI successfully destroyed")
	return nil
}

func destroyNodes(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		if !destroyAll {
			return fmt.Errorf("to destroy all existing clusters created by Avalanche CLI, call avalanche node destroy --all. To destroy a specified cluster, call avalanche node destroy CLUSTERNAME")
		}
		return Cleanup()
	}
	clusterName := args[0]
	if err := nodePkg.CheckCluster(app, clusterName); err != nil {
		return err
	}
	isExternalCluster, err := checkClusterExternal(clusterName)
	if err != nil {
		return err
	}
	if authorizeAll {
		authorizeAccess = true
		authorizeRemove = true
	}
	if err := getDeleteConfigConfirmation(); err != nil {
		return err
	}
	nodesToStop, err := nodePkg.GetClusterNodes(app, clusterName)
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
	nodeErrors := map[string]error{}
	cloudSecurityGroupList, err := getCloudSecurityGroupList(nodesToStop)
	if err != nil {
		return err
	}
	firstAvailableNodes, noAvailableNodesFound := getFirstAvailableNode(nodesToStop)
	if noAvailableNodesFound {
		return removeClustersConfigFiles(clusterName)
	}
	nodeToStopConfig, err := app.LoadClusterNodeConfig(firstAvailableNodes)
	if err != nil {
		return err
	}
	// TODO: will need to change this logic if we decide to mix AWS and GCP instances in a cluster
	filteredSGList := utils.Filter(cloudSecurityGroupList, func(sg regionSecurityGroup) bool { return sg.cloud == nodeToStopConfig.CloudService })
	if len(filteredSGList) == 0 {
		return fmt.Errorf("no endpoint found in the  %s", nodeToStopConfig.CloudService)
	}
	var gcpCloud *gcpAPI.GcpCloud
	ec2SvcMap := make(map[string]*awsAPI.AwsCloud)
	// TODO: need implementation for GCP
	if nodeToStopConfig.CloudService == constants.AWSCloudService {
		for _, sg := range filteredSGList {
			sgEc2Svc, err := awsAPI.NewAwsCloud(awsProfile, sg.region)
			if err != nil {
				return err
			}
			ec2SvcMap[sg.region] = sgEc2Svc
		}
	}
	for _, node := range nodesToStop {
		if !isExternalCluster {
			// if we can't find node config path, that means node already deleted on console
			// but we didn't get to delete the node from cluster config file
			if !utils.FileExists(app.GetNodeConfigPath(node)) {
				continue
			}
			nodeConfig, err := app.LoadClusterNodeConfig(node)
			if err != nil {
				nodeErrors[node] = err
				ux.Logger.RedXToUser("Failed to destroy node %s due to %s", node, err.Error())
				continue
			}
			if nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService {
				if !(authorizeAccess || nodePkg.AuthorizedAccessFromSettings(app)) && (requestCloudAuth(constants.AWSCloudService) != nil) {
					return fmt.Errorf("cloud access is required")
				}
				if err = ec2SvcMap[nodeConfig.Region].DestroyAWSNode(nodeConfig, clusterName); err != nil {
					if isExpiredCredentialError(err) {
						ux.Logger.PrintToUser("")
						printExpiredCredentialsOutput(awsProfile)
						return fmt.Errorf("invalid cloud credentials")
					}
					if !errors.Is(err, awsAPI.ErrNodeNotFoundToBeRunning) {
						nodeErrors[node] = err
						continue
					}
					ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
				}
				for _, sg := range filteredSGList {
					if err = deleteHostSecurityGroupRule(ec2SvcMap[sg.region], nodeConfig.ElasticIP, sg.securityGroup); err != nil {
						ux.Logger.RedXToUser("unable to delete IP address %s from security group %s in region %s due to %s, please delete it manually",
							nodeConfig.ElasticIP, sg.securityGroup, sg.region, err.Error())
					}
				}
			} else {
				if !(authorizeAccess || nodePkg.AuthorizedAccessFromSettings(app)) && (requestCloudAuth(constants.GCPCloudService) != nil) {
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
					ux.Logger.GreenCheckmarkToUser("node %s is already destroyed", nodeConfig.NodeID)
				}
			}
			ux.Logger.GreenCheckmarkToUser("Node instance %s in cluster %s successfully destroyed!", nodeConfig.NodeID, clusterName)
		}
		if err := removeDeletedNodeDirectory(node); err != nil {
			ux.Logger.RedXToUser("Failed to delete node config for node %s due to %s", node, err.Error())
			return err
		}
	}
	if len(nodeErrors) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		invalidCloudCredentials := false
		for node, nodeErr := range nodeErrors {
			if strings.Contains(nodeErr.Error(), constants.ErrReleasingGCPStaticIP) {
				ux.Logger.RedXToUser("Node is destroyed, but failed to release static ip address for node %s due to %s", node, nodeErr)
			} else {
				if strings.Contains(nodeErr.Error(), "AuthFailure") {
					invalidCloudCredentials = true
				}
				ux.Logger.RedXToUser("Failed to destroy node %s due to %s", node, nodeErr)
			}
		}
		if invalidCloudCredentials {
			return fmt.Errorf("failed to destroy node(s) due to invalid cloud credentials %s", maps.Keys(nodeErrors))
		}
		return fmt.Errorf("failed to destroy node(s) %s", maps.Keys(nodeErrors))
	} else {
		if isExternalCluster {
			ux.Logger.GreenCheckmarkToUser("All nodes in EXTERNAL cluster %s are successfully removed!", clusterName)
		} else {
			ux.Logger.GreenCheckmarkToUser("All nodes in cluster %s are successfully destroyed!", clusterName)
		}
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

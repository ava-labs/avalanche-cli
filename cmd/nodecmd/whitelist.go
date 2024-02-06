// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/net/context"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type regionSecurityGroup struct {
	cloud         string
	region        string
	securityGroup string
}

func newWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist <clusterName> [IP]",
		Short: "(ALPHA Warning) Whitelist IP for access to all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node whitelist command adds IP to cloud security access rules allowing it to access
all nodes in the cluster via ssh or http.`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         whitelistIP,
	}
	return cmd
}

func whitelistIP(_ *cobra.Command, args []string) error {
	var err error
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	userIPAddress := ""
	if len(args) > 1 {
		userIPAddress = args[1]
	} else {
		userIPAddress, err = utils.GetUserIPAddress()
		if err != nil {
			return fmt.Errorf("failed to get user IP address")
		}
		ux.Logger.PrintToUser("No IP provided. Detected your IP address as %s.", userIPAddress)
	}
	if !utils.IsValidIP(userIPAddress) {
		return fmt.Errorf("invalid IP address: %s", userIPAddress)
	}
	cloudSecurityGroupList := []regionSecurityGroup{}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			ux.Logger.PrintToUser("Failed to parse node %s due to %s", node, err.Error())
			return err
		}
		if slices.Contains(cloudSecurityGroupList, regionSecurityGroup{
			cloud:         nodeConfig.CloudService,
			region:        nodeConfig.Region,
			securityGroup: nodeConfig.SecurityGroup,
		}) {
			continue
		}
		cloudSecurityGroupList = append(cloudSecurityGroupList, regionSecurityGroup{
			cloud:         nodeConfig.CloudService,
			region:        nodeConfig.Region,
			securityGroup: nodeConfig.SecurityGroup,
		})
	}
	if len(cloudSecurityGroupList) == 0 {
		ux.Logger.PrintToUser("No nodes found in cluster %s", clusterName)
		return fmt.Errorf("no nodes found in cluster %s", clusterName)
	}

	// GCP doesn't have regions  so we need to reduce it to only list of security groups
	gcpSGFound := false
	// whitelist IP
	for _, cloudSecurityGroup := range cloudSecurityGroupList {
		if cloudSecurityGroup.cloud == constants.GCPCloudService {
			gcpSGFound = true
			continue
		}
		ux.Logger.PrintToUser("Whitelisting IP %s in %s cloud region %s", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
		if cloudSecurityGroup.cloud == "" || cloudSecurityGroup.cloud == constants.AWSCloudService {
			ec2Svc, err := awsAPI.NewAwsCloud(awsProfile, cloudSecurityGroup.region)
			if err != nil {
				ux.Logger.PrintToUser("Failed to establish connection to %s cloud region %s", cloudSecurityGroup.cloud, cloudSecurityGroup.region)
				return err
			}
			securityGroupExists, sg, err := ec2Svc.CheckSecurityGroupExists(cloudSecurityGroup.securityGroup)
			if err != nil || !securityGroupExists {
				ux.Logger.PrintToUser("Can't find security group %s in %s cloud region %s", cloudSecurityGroup.securityGroup, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
				return err
			}
			ipInTCP := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.SSHTCPPort)
			ipInHTTP := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.AvalanchegoAPIPort)
			if ipInTCP {
				ux.Logger.PrintToUser("IP %s is already whitelisted in %s cloud region %s for ssh access. Skipping...", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
			} else {
				if err := ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", userIPAddress, constants.SSHTCPPort); err != nil {
					ux.Logger.PrintToUser("Failed to whitelist IP %s in %s cloud region %s for ssh access", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
					return err
				}
			}
			if ipInHTTP {
				ux.Logger.PrintToUser("IP %s is already whitelisted in %s cloud region %s for http access. Skipping...", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
			} else {
				if err := ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", userIPAddress, constants.AvalanchegoAPIPort); err != nil {
					ux.Logger.PrintToUser("Failed to whitelist IP %s in %s cloud region %s for http access", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
					return err
				}
			}
		}
	}
	if gcpSGFound {
		prefix, err := defaultAvalancheCLIPrefix("")
		if err != nil {
			return err
		}
		networkName := fmt.Sprintf("%s-network", prefix)
		gcpClient, projectName, _, err := getGCPCloudCredentials()
		if err != nil {
			return err
		}
		gcpCloud, err := gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Whitelisting IP %s in %s cloud", userIPAddress, constants.GCPCloudService)
		if _, err = gcpCloud.SetFirewallRule(userIPAddress, fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(userIPAddress, ".", "")), networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort)}); err != nil {
			if errors.IsAlreadyExists(err) {
				ux.Logger.PrintToUser("IP %s is already whitelisted in %s cloud. Skipping...", userIPAddress, constants.GCPCloudService)
				return nil
			} else {
				ux.Logger.PrintToUser("Failed to whitelist IP %s in %s cloud", userIPAddress, constants.GCPCloudService)
				return err
			}
		}
	}
	return nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
)

var (
	app           *application.Avalanche
	userIPAddress string
	userPubKey    string
	discoverIP    bool
)

func newWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist <clusterName> [--ip <IP>] [--ssh \"<sshPubKey>\"]",
		Short: "(ALPHA Warning) Grant access to the cluster ",
		Long: `(ALPHA Warning) The whitelist command suite provides a collection of tools for granting access to the cluster.

	Command adds IP if --ip params provided to cloud security access rules allowing it to access all nodes in the cluster via ssh or http.
	It also command adds SSH public key to all nodes in the cluster if --ssh params is there.
	If no params provided it detects current user IP automaticaly and whitelists it`,
		Args: cobrautils.MinimumNArgs(1),
		RunE: whitelist,
	}
	cmd.Flags().StringVar(&userIPAddress, "ip", "", "ip address to whitelist")
	cmd.Flags().StringVar(&userPubKey, "ssh", "", "ssh public key to whitelist")
	cmd.Flags().BoolVarP(&discoverIP, "current-ip", "y", false, "whitelist current host ip")
	return cmd
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
type regionSecurityGroup struct {
	cloud         string
	region        string
	securityGroup string
}

func whitelist(_ *cobra.Command, args []string) error {
	var err error
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := failForExternal(clusterName); err != nil {
		return err
	}

	if discoverIP {
		userIPAddress, err = utils.GetUserIPAddress()
		if err != nil {
			return fmt.Errorf("failed to get user IP address")
		}
		ux.Logger.PrintToUser("Detected your IP address as: %s", logging.LightBlue.Wrap(userIPAddress))
	}
	if userIPAddress == "" && userPubKey == "" {
		// prompt for ssh key
		userPubKey, err = utils.ReadLongString("Enter SSH public key to whitelist (leave empty to skip):\n")
		if err != nil {
			return err
		}
		// prompt for IP
		detectedIPAddress, err := utils.GetUserIPAddress()
		if err != nil {
			return fmt.Errorf("failed to get user IP address")
		}
		ux.Logger.PrintToUser("Detected your IP address as: %s", logging.LightBlue.Wrap(detectedIPAddress))
		userIPAddress, err = app.Prompt.CaptureStringAllowEmpty(fmt.Sprintf("Enter IP address to whitelist (Also you can press Enter to use %s or S to skip)", logging.LightBlue.Wrap(detectedIPAddress)))
		if err != nil {
			return err
		}
		if userIPAddress == "" {
			userIPAddress = detectedIPAddress
		}
		if strings.ToLower(userIPAddress) == "s" {
			userIPAddress = ""
		}
	}
	if userPubKey != "" {
		if !utils.IsSSHPubKey(userPubKey) {
			return fmt.Errorf("invalid SSH public key: %s", userPubKey)
		}
		if err := whitelistSSHPubKey(clusterName, userPubKey); err != nil {
			return err
		}
		if userIPAddress == "" {
			return nil // if only ssh key is provided, no need to whitelist IP
		}
		ux.Logger.PrintLineSeparator()
	}
	if userIPAddress != "" && !utils.IsValidIP(userIPAddress) {
		return fmt.Errorf("invalid IP address: %s", userIPAddress)
	}
	if userIPAddress != "" {
		ux.Logger.GreenCheckmarkToUser("Whitelisting IP: %s", logging.LightBlue.Wrap(userIPAddress))
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
			ux.Logger.RedXToUser("No nodes found in cluster %s", clusterName)
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
			ux.Logger.GreenCheckmarkToUser("Whitelisting IP %s in %s cloud region %s", userIPAddress, cloudSecurityGroup.cloud, cloudSecurityGroup.region)
			if cloudSecurityGroup.cloud == "" || cloudSecurityGroup.cloud == constants.AWSCloudService {
				if cloudSecurityGroup.cloud == "" || cloudSecurityGroup.cloud == constants.AWSCloudService {
					if err := GrantAccessToIPinAWS(awsProfile, cloudSecurityGroup.region, cloudSecurityGroup.securityGroup, userIPAddress); err != nil {
						return err
					}
				}
			}
		}
		if gcpSGFound {
			ux.Logger.GreenCheckmarkToUser("Whitelisting IP %s in %s cloud", userIPAddress, constants.GCPCloudService)
			if err := GrantAccessToIPinGCP(userIPAddress); err != nil {
				return err
			}
		}
	}
	return nil
}

func GrantAccessToIPinAWS(awsProfile string, region string, sgName string, userIPAddress string) error {
	ec2Svc, err := awsAPI.NewAwsCloud(awsProfile, region)
	if err != nil {
		return fmt.Errorf("failed to establish connection to %s cloud region %s with err: %w", constants.AWSCloudService, region, err)
	}
	securityGroupExists, sg, err := ec2Svc.CheckSecurityGroupExists(sgName)
	if err != nil || !securityGroupExists {
		return fmt.Errorf("can't find security group %s in %s cloud region %s with err: %w", sgName, constants.AWSCloudService, region, err)
	}
	ipInTCP := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.SSHTCPPort)
	ipInHTTP := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.AvalanchegoAPIPort)
	ipInGrafana := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.AvalanchegoGrafanaPort)
	if ipInTCP {
		ux.Logger.RedXToUser("IP %s is already whitelisted in %s cloud region %s for ssh access. Skipping...", userIPAddress, constants.AWSCloudService, region)
	} else {
		if err := ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", userIPAddress, constants.SSHTCPPort); err != nil {
			return fmt.Errorf("failed to whitelist IP %s in %s cloud region %s for ssh access with err: %w", userIPAddress, constants.AWSCloudService, region, err)
		}
	}
	if ipInHTTP {
		ux.Logger.RedXToUser("IP %s is already whitelisted in %s cloud region %s for http access. Skipping...", userIPAddress, constants.AWSCloudService, region)
	} else {
		if err := ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", userIPAddress, constants.AvalanchegoAPIPort); err != nil {
			return fmt.Errorf("failed to whitelist IP %s in %s cloud region %s for http access with err: %w", userIPAddress, constants.AWSCloudService, region, err)
		}
	}
	if ipInGrafana {
		ux.Logger.RedXToUser("IP %s is already whitelisted in %s cloud region %s for grafana access. Skipping...", userIPAddress, constants.AWSCloudService, region)
	} else {
		if err := ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", userIPAddress, constants.AvalanchegoGrafanaPort); err != nil {
			return fmt.Errorf("failed to whitelist IP %s in %s cloud region %s for grafana access with err: %w", userIPAddress, constants.AWSCloudService, region, err)
		}
	}
	return nil
}

func GrantAccessToIPinGCP(userIPAddress string) error {
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
	if _, err = gcpCloud.SetFirewallRule(userIPAddress, fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(userIPAddress, ".", "")), networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort), strconv.Itoa(constants.AvalanchegoGrafanaPort)}); err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("IP %s is already whitelisted in %s cloud. Skipping... ", userIPAddress, constants.GCPCloudService)
		} else {
			return fmt.Errorf("failed to whitelist IP %s in %s cloud with err: %w", userIPAddress, constants.GCPCloudService, err)
		}
	}
	return nil
}

func whitelistSSHPubKey(clusterName string, pubkey string) error {
	sshPubKey := strings.Trim(pubkey, "\"'")
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	if clusterConfig.MonitoringInstance != "" {
		monitoringHost, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetMonitoringInventoryDir(clusterName))
		if err != nil {
			return err
		}
		hosts = append(hosts, monitoringHost...)
	}
	if len(clusterConfig.LoadTestInstance) != 0 {
		loadTestHost, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetLoadTestInventoryDir(clusterName))
		if err != nil {
			return err
		}
		hosts = append(hosts, loadTestHost...)
	}
	ux.Logger.PrintToUser("Whitelisting SSH public key on all nodes in cluster: %s", logging.LightBlue.Wrap(clusterName))
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *host.Host) {
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

// getCloudSecurityGroupList returns a list of cloud security groups for a given cluster nodes
func getCloudSecurityGroupList(clusterNodes []string) ([]regionSecurityGroup, error) {
	cloudSecurityGroupList := []regionSecurityGroup{}
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			ux.Logger.PrintToUser("Failed to parse node %s due to %s", node, err.Error())
			return nil, err
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
	return cloudSecurityGroupList, nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	loadTestScriptPath string
	loadTestScriptArgs string
)

func newLoadTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loadtest [clusterName]",
		Short: "(ALPHA Warning) Start loadtest for existing devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command starts a loadtest run for the existing devnet cluster. It creates a separated cloud server and
this loadtest script will be run on the provisioned cloud server in the same cloud and region as the cluster.
Loadtest script will be run in ubuntu user home directory with the provided arguments.
After loadtest is done it will deliver generated reports if any along with loadtest logs before terminating used cloud server.`,

		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         createLoadTest,
	}
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create loadtest node in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create loadtest in GCP cloud")
	cmd.Flags().StringVar(&nodeType, "node-type", "", "cloud instance type for loadtest script. Use 'default' to use recommended default instance type")
	cmd.Flags().StringVar(&loadTestScriptPath, "loadtest-script-path", "", "loadtest script path")
	cmd.Flags().StringVar(&loadTestScriptArgs, "loadtest-script-args", "", "loadtest script arguments")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	cmd.Flags().BoolVar(&useSSHAgent, "use-ssh-agent", false, "use ssh agent(ex: Yubikey) for ssh auth")
	cmd.Flags().StringVar(&sshIdentity, "ssh-agent-identity", "", "use given ssh identity(only for ssh agent). If not set, default will be used")
	return cmd
}

func preLoadTestChecks(clusterName string) error {
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if useAWS && useGCP {
		return fmt.Errorf("could not use both AWS and GCP cloud options")
	}
	if !useAWS && awsProfile != constants.AWSDefaultCredential {
		return fmt.Errorf("could not use AWS profile for non AWS cloud option")
	}
	if sshIdentity != "" && !useSSHAgent {
		return fmt.Errorf("could not use ssh identity without using ssh agent")
	}
	if useSSHAgent && !utils.IsSSHAgentAvailable() {
		return fmt.Errorf("ssh agent is not available")
	}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	if len(clusterNodes) == 0 {
		return fmt.Errorf("no nodes found for loadtesting in the cluster %s", clusterName)
	}
	return nil
}

func createLoadTest(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	preLoadTestChecks(clusterName)
	clustersConfig, err := app.LoadClustersConfig()
	loadTestNodeConfig := models.RegionConfig{}
	loadTestCloudConfig := models.CloudConfig{}
	if err != nil {
		return err
	}
	if clustersConfig.Clusters[clusterName].Network.Kind != models.Devnet {
		return fmt.Errorf("node loadtest command can be applied to devnet clusters only")
	}
	if loadTestScriptPath == "" {
		loadTestScriptPath = "loadtest.sh"
	}
	// set ssh-Key
	if useSSHAgent && sshIdentity == "" {
		sshIdentity, err = setSSHIdentity()
		if err != nil {
			return err
		}
	}

	cloudService, err := setCloudService()
	if err != nil {
		return err
	}
	nodeType, err = setCloudInstanceType(cloudService)
	if err != nil {
		return err
	}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	fmt.Printf("cluster nodes %s\n", clusterNodes)
	cloudSecurityGroupList, err := getCloudSecurityGroupList(clusterNodes)
	if err != nil {
		return err
	}
	// we only support endpoint in the same cluster
	filteredSGList := utils.Filter(cloudSecurityGroupList, func(sg regionSecurityGroup) bool { return sg.cloud == cloudService })
	if len(filteredSGList) == 0 {
		return fmt.Errorf("no endpoint found in the  %s", cloudService)
	}
	loadTestRegion := filteredSGList[0].region
	// create loadtest cloud server
	if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(cloudService) != nil) {
		return fmt.Errorf("cloud access is required")
	}
	if cloudService == constants.AWSCloudService {
		ec2SvcMap, ami, _, err := getAWSCloudConfig(awsProfile)
		if err != nil {
			return err
		}
		loadTestEc2SvcMap := make(map[string]*awsAPI.AwsCloud)
		loadTestEc2SvcMap[loadTestRegion] = ec2SvcMap[loadTestRegion]
		loadTestCloudConfig, err = createAWSInstances(loadTestEc2SvcMap, nodeType, map[string]int{loadTestRegion: 1}, []string{loadTestRegion}, ami, true)
		if err != nil {
			return err
		}
		loadTestNodeConfig = loadTestCloudConfig[loadTestRegion]
		// get loadtest public IP
		loadTestPublicIPMap, err := loadTestEc2SvcMap[loadTestRegion].GetInstancePublicIPs(loadTestNodeConfig.InstanceIDs)
		if err != nil {
			return err
		}
		loadTestNodeConfig.PublicIPs = []string{loadTestPublicIPMap[loadTestNodeConfig.InstanceIDs[0]]}
		for _, sg := range filteredSGList {
			if err = grantAccessToPublicIPViaSecurityGroup(ec2SvcMap[sg.region], loadTestNodeConfig.PublicIPs[0], sg.securityGroup, sg.region); err != nil {
				return err
			}
		}

		// whitelist access from loadtest node to cluster nodes
	} else if cloudService == constants.GCPCloudService {
		// Get GCP Credential, zone, Image ID, service account key file path, and GCP project name
		gcpClient, _, imageID, _, _, err := getGCPConfig()
		if err != nil {
			return err
		}
		loadTestCloudConfig, err = createGCPInstance(gcpClient, nodeType, map[string]int{loadTestRegion: 1}, imageID, clusterName, true)
		if err != nil {
			return err
		}
		loadTestNodeConfig = loadTestCloudConfig[loadTestRegion]
		//loadTestPublicIPMap, err := gcpClient.GetInstancePublicIPs(loadTestRegion, loadTestNodeConfig.InstanceIDs)
		//if err != nil {
		//	return err
		//}
		//loadTestNodeConfig.PublicIPs = []string{loadTestPublicIPMap[loadTestNodeConfig.InstanceIDs[0]]}
		//if err = grantAccessToPublicIPViaFirewall(gcpClient, projectName, loadTestNodeConfig.PublicIPs[0], "loadtest"); err != nil {
		//	return err
		//}
	} else {
		return fmt.Errorf("cloud service %s is not supported", cloudService)
	}

	// deploy loadtest script
	ansibleInstanceID, err := models.HostCloudIDToAnsibleID(cloudService, loadTestNodeConfig.InstanceIDs[0])
	loadTestHost := models.Host{
		NodeID:            ansibleInstanceID,
		IP:                loadTestNodeConfig.PublicIPs[0],
		SSHUser:           constants.AnsibleSSHUser,
		SSHPrivateKeyPath: loadTestCloudConfig[loadTestRegion].CertFilePath,
		SSHCommonArgs:     constants.AnsibleSSHUseAgentParams,
	}

	failedHosts := waitForHosts([]*models.Host{&loadTestHost})
	if failedHosts.Len() > 0 {
		for _, result := range failedHosts.GetResults() {
			ux.Logger.PrintToUser("Loadtest instance %s failed to provision with error %s. Please check instance logs for more information", result.NodeID, result.Err)
		}
		return fmt.Errorf("failed to provision node(s) %s", failedHosts.GetNodeList())
	}
	ux.Logger.PrintToUser("Loadtest instance %s provisioned successfully", loadTestHost.NodeID)
	// run loadtest script
	//ltScript := ""
	//if ltScript, err = ssh.RunSSHSetupLoadTest(&loadTestHost, loadTestScriptPath); err != nil {
	//	return err
	//}
	//ux.Logger.PrintToUser("Loadtest instance %s ready", loadTestHost.NodeID)
	//if err := ssh.RunSSHStartLoadTest(&loadTestHost, ltScript, loadTestScriptArgs); err != nil {
	//	return err
	//}
	ux.Logger.PrintToUser("Loadtest instance %s is done", loadTestHost.NodeID)

	return nil
}

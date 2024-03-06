// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package root

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	avalanchegoVersion = "v1.10.18"
	network            = "fuji"
	networkCapitalized = "Fuji"
	numNodes           = 1
)

var (
	hostName string
	NodeID   string
)

var _ = ginkgo.Describe("[Node create]", func() {
	ginkgo.It("can create a node", func() {
		output := commands.NodeCreate(network, avalanchegoVersion, numNodes, false, 0, commands.ExpectSuccess)
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!"))
		// parse hostName
		re := regexp.MustCompile(`Generated staking keys for host (\S+)\[NodeID-(\S+)\]`)
		match := re.FindStringSubmatch(output)
		if len(match) >= 3 {
			hostName = match[1]
			NodeID = fmt.Sprintf("NodeID-%s", match[2])
		} else {
			ginkgo.Fail("failed to parse hostName and NodeID")
		}
	})
	ginkgo.It("creates cluster config", func() {
		usr, err := user.Current()
		gomega.Expect(err).Should(gomega.BeNil())
		homeDir := usr.HomeDir
		relativePath := "nodes"
		content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, constants.ClustersConfigFileName))
		gomega.Expect(err).Should(gomega.BeNil())
		clustersConfig := models.ClustersConfig{}
		err = json.Unmarshal(content, &clustersConfig)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(clustersConfig.Clusters).To(gomega.HaveLen(1))
		gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName].Network.Kind.String()).To(gomega.Equal(networkCapitalized))
		gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName].Nodes).To(gomega.HaveLen(numNodes))
	})
	ginkgo.It("creates node config", func() {
		fmt.Println("HostName: ", hostName)
		usr, err := user.Current()
		gomega.Expect(err).Should(gomega.BeNil())
		homeDir := usr.HomeDir
		relativePath := "nodes"
		content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, hostName, "node_cloud_config.json"))
		gomega.Expect(err).Should(gomega.BeNil())
		nodeCloudConfig := models.NodeConfig{}
		err = json.Unmarshal(content, &nodeCloudConfig)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(nodeCloudConfig.NodeID).To(gomega.Equal(hostName))
		gomega.Expect(nodeCloudConfig.ElasticIP).To(gomega.ContainSubstring(constants.E2ENetworkPrefix))
		gomega.Expect(nodeCloudConfig.CertPath).To(gomega.ContainSubstring(homeDir))
		gomega.Expect(nodeCloudConfig.UseStaticIP).To(gomega.Equal(false))
	})
	ginkgo.It("installs and runs avalanchego", func() {
		avalancegoProcess := commands.NodeSSH(constants.E2EClusterName, "ps -elf")
		gomega.Expect(avalancegoProcess).To(gomega.ContainSubstring("/home/ubuntu/avalanche-node/avalanchego"))
	})
	ginkgo.It("installs latest version of avalanchego", func() {
		avalanchegoVersionClean := strings.TrimPrefix(avalanchegoVersion, "v")
		avalancegoVersion := commands.NodeSSH(constants.E2EClusterName, "/home/ubuntu/avalanche-node/avalanchego --version")
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("go="))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("avalanchego/" + avalanchegoVersionClean))
	})
	ginkgo.It("configured avalanchego", func() {
		avalancegoConfig := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/node.json")
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("\"network-id\": \"" + network + "\""))
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("public-ip"))
		avalancegoConfigCChain := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/chains/C/config.json")
		gomega.Expect(avalancegoConfigCChain).To(gomega.ContainSubstring("\"state-sync-enabled\": true"))
	})
	ginkgo.It("can get cluster status", func() {
		output := commands.NodeStatus()
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("Checking if node(s) are bootstrapped to Primary Network"))
		gomega.Expect(output).To(gomega.ContainSubstring("Checking if node(s) are healthy"))
		gomega.Expect(output).To(gomega.ContainSubstring("Getting avalanchego version of node(s)"))
		gomega.Expect(output).To(gomega.ContainSubstring(constants.E2ENetworkPrefix))
		gomega.Expect(output).To(gomega.ContainSubstring(hostName))
		gomega.Expect(output).To(gomega.ContainSubstring(NodeID))
		gomega.Expect(output).To(gomega.ContainSubstring(networkCapitalized))
	})
	ginkgo.It("can ssh to a created node", func() {
		output := commands.NodeSSH(constants.E2EClusterName, "echo hello")
		gomega.Expect(output).To(gomega.ContainSubstring("hello"))
	})
	ginkgo.It("can list created nodes", func() {
		output := commands.NodeList()
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring(networkCapitalized))
		gomega.Expect(output).To(gomega.ContainSubstring("docker1"))
		gomega.Expect(output).To(gomega.ContainSubstring("NodeID"))
		gomega.Expect(output).To(gomega.ContainSubstring(constants.E2ENetworkPrefix))
	})
	ginkgo.It("logged operations", func() {
		logs := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/logs/main.log")
		gomega.Expect(logs).To(gomega.ContainSubstring("initializing node"))
		gomega.Expect(logs).To(gomega.ContainSubstring("initializing API server"))
		gomega.Expect(logs).To(gomega.ContainSubstring("creating leveldb"))
		gomega.Expect(logs).To(gomega.ContainSubstring("initializing database"))
		gomega.Expect(logs).To(gomega.ContainSubstring("creating proposervm wrapper"))
		gomega.Expect(logs).To(gomega.ContainSubstring("check started passing"))
	})
	ginkgo.It("can upgrade the nodes", func() {
		output := commands.NodeUpgrade()
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("Upgrading Avalanche Go"))
		latestAvagoVersion := strings.TrimPrefix(commands.GetLatestAvagoVersionFromGithub(), "v")
		avalanchegoVersion := commands.NodeSSH(constants.E2EClusterName, "/home/ubuntu/avalanche-node/avalanchego --version")
		gomega.Expect(avalanchegoVersion).To(gomega.ContainSubstring("go="))
		gomega.Expect(avalanchegoVersion).To(gomega.ContainSubstring("avalanchego/" + latestAvagoVersion))
	})
	ginkgo.It("can whitelist ssh", func() {
		output := commands.NodeWhitelistSSH("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC test@localhost")
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("Whitelisted SSH public key"))
		authorizedFile := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.ssh/authorized_keys")
		gomega.Expect(authorizedFile).To(gomega.ContainSubstring("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC test@localhost"))
	})
	ginkgo.It("can cleanup", func() {
		commands.DeleteE2EInventory()
		commands.DeleteE2ECluster()
		commands.DeleteNode(hostName)
	})
})

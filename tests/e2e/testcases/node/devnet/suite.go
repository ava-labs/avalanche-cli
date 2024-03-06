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

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var (
	hostName    string
	NodeID      string
	apiHostName string
	apiNodeID   string
)

const (
	NumNodes    = 1
	NumAPINodes = 1
)

var _ = ginkgo.Describe("[Node devnet]", func() {
	ginkgo.It("can create a node", func() {
		output := commands.NodeDevnet(NumNodes, NumAPINodes)
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!"))
		// parse hostName
		re := regexp.MustCompile(`Generated staking keys for host (\S+)\[NodeID-(\S+)\]`)
		matches := re.FindAllStringSubmatch(output, -1)
		// expect 2 nodes 1 validator and 1 api node
		gomega.Expect(len(matches)).To(gomega.Equal(NumNodes + NumAPINodes))
		gomega.Expect(NumAPINodes + NumNodes).To(gomega.Equal(2)) // 1 validator and 1 api node or logic of the test is wrong
		match := matches[0]                                       // validator node
		if len(match) >= 3 {
			hostName = match[1]
			NodeID = fmt.Sprintf("NodeID-%s", match[2])
		} else {
			ginkgo.Fail("failed to parse hostName and NodeID")
		}
		match = matches[1] // api node
		if len(match) >= 3 {
			apiHostName = match[1]
			apiNodeID = fmt.Sprintf("NodeID-%s", match[2])
		} else {
			ginkgo.Fail("[API]failed to parse hostName and NodeID")
		}
	})
	ginkgo.It("has correct cluster config record for API", func() {
		usr, err := user.Current()
		gomega.Expect(err).Should(gomega.BeNil())
		homeDir := usr.HomeDir
		relativePath := "nodes"
		content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, constants.ClustersConfigFileName))
		gomega.Expect(err).Should(gomega.BeNil())
		clustersConfig := models.ClustersConfig{}
		err = json.Unmarshal(content, &clustersConfig)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName].APINodes).To(gomega.HaveLen(NumAPINodes))
	})
	ginkgo.It("installs and runs avalanchego", func() {
		avalancegoVersion := commands.NodeSSH(constants.E2EClusterName, "/home/ubuntu/avalanche-node/avalanchego --version")
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("avalanchego/"))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("[database="))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("rpcchainvm="))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("go="))
		avalancegoProcess := commands.NodeSSH(constants.E2EClusterName, "ps -elf")
		gomega.Expect(avalancegoProcess).To(gomega.ContainSubstring("/home/ubuntu/avalanche-node/avalanchego"))
	})
	ginkgo.It("configured avalanchego", func() {
		avalancegoConfig := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/node.json")
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("\"genesis-file\": \"/home/ubuntu/.avalanchego/configs/genesis.json\""))
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("\"network-id\": \"network-1338\""))
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("\"public-ip\": \"" + constants.E2ENetworkPrefix))
		avalancegoConfigCChain := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/chains/C/config.json")
		gomega.Expect(avalancegoConfigCChain).To(gomega.ContainSubstring("\"state-sync-enabled\": true"))
	})
	ginkgo.It("provides avalanchego with staking certs", func() {
		stakingFiles := commands.NodeSSH(constants.E2EClusterName, "ls /home/ubuntu/.avalanchego/staking/")
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("signer.key"))
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("staker.crt"))
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("staker.key"))
	})
	ginkgo.It("provides avalanchego with genesis", func() {
		genesisFile := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/genesis.json")
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("avaxAddr"))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("initialStakers"))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("cChainGenesis"))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring(NodeID))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("\"rewardAddress\": \"X-custom"))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("\"startTime\":"))
		gomega.Expect(genesisFile).To(gomega.ContainSubstring("\"networkID\": 1338,"))
		// make sure there is no API node in the genesis
		gomega.Expect(genesisFile).To(gomega.Not(gomega.ContainSubstring(apiNodeID)))
	})
	ginkgo.It("installs and configures avalanche-cli on the node ", func() {
		stakingFiles := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanche-cli/config.json")
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("\"metricsenabled\": false"))
		avalanceCliVersion := commands.NodeSSH(constants.E2EClusterName, "/home/ubuntu/bin/avalanche --version")
		gomega.Expect(avalanceCliVersion).To(gomega.ContainSubstring("avalanche version"))
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
		gomega.Expect(output).To(gomega.ContainSubstring(apiHostName))
		gomega.Expect(output).To(gomega.ContainSubstring(apiNodeID))
		gomega.Expect(output).To(gomega.ContainSubstring("Devnet"))
	})
	ginkgo.It("can ssh to a created node", func() {
		output := commands.NodeSSH(constants.E2EClusterName, "echo hello")
		gomega.Expect(output).To(gomega.ContainSubstring("hello"))
	})
	ginkgo.It("can list created nodes", func() {
		output := commands.NodeList()
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("Devnet"))
		gomega.Expect(output).To(gomega.ContainSubstring("docker1"))
		gomega.Expect(output).To(gomega.ContainSubstring("NodeID"))
		gomega.Expect(output).To(gomega.ContainSubstring(constants.E2ENetworkPrefix))
	})
	ginkgo.It("can cleanup", func() {
		commands.DeleteE2EInventory()
		commands.DeleteE2ECluster()
		commands.DeleteNode(hostName)
	})
})

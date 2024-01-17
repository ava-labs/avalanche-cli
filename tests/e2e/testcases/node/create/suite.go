// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package root

import (
	"fmt"
	"regexp"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var (
	hostName string
	NodeID   string
)

var _ = ginkgo.Describe("[Node create]", func() {
	ginkgo.It("can create a node", func() {
		output := commands.NodeCreate("fuji", 1)
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
	ginkgo.It("installs and runs avalanchego", func() {
		avalancegoVersion := commands.NodeSSH(constants.E2EClusterName, "/home/ubuntu/avalanche-node/avalanchego --version")
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("avalanche/"))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("[database="))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("rpcchainvm="))
		gomega.Expect(avalancegoVersion).To(gomega.ContainSubstring("go="))
		avalancegoProcess := commands.NodeSSH(constants.E2EClusterName, "ps -elf")
		gomega.Expect(avalancegoProcess).To(gomega.ContainSubstring("/home/ubuntu/avalanche-node/avalanchego"))
	})
	ginkgo.It("configured avalanchego", func() {
		avalancegoConfig := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/node.json")
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("\"network-id\": \"fuji\""))
		gomega.Expect(avalancegoConfig).To(gomega.ContainSubstring("public-ip"))
		avalancegoConfigCChain := commands.NodeSSH(constants.E2EClusterName, "cat /home/ubuntu/.avalanchego/configs/chains/C/config.json")
		gomega.Expect(avalancegoConfigCChain).To(gomega.ContainSubstring("\"state-sync-enabled\": true"))
	})
	ginkgo.It("provides avalanchego with staking certs", func() {
		stakingFiles := commands.NodeSSH(constants.E2EClusterName, "ls /home/ubuntu/.avalanchego/staking/")
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("signer.key"))
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("staker.crt"))
		gomega.Expect(stakingFiles).To(gomega.ContainSubstring("staker.key"))
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
		gomega.Expect(output).To(gomega.ContainSubstring("Fuji"))
	})
	ginkgo.It("can ssh to a created node", func() {
		output := commands.NodeSSH(constants.E2EClusterName, "echo hello")
		gomega.Expect(output).To(gomega.ContainSubstring("hello"))
	})
	ginkgo.It("can list created nodes", func() {
		output := commands.NodeList()
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("Fuji"))
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
})

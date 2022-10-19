// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName  = "e2eSubnetTest"
	controlKeys = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	keyName     = "ewoq"
)

var _ = ginkgo.Describe("[Public Subnet]", func() {
	ginkgo.BeforeEach(func() {
		// local network
		_ = commands.StartNetwork()
		// key
		_ = utils.DeleteKey(keyName)
		output, err := commands.CreateKeyFromPath(keyName, utils.EwoqKeyPath)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		// subnet config
		_ = utils.DeleteConfigs(subnetName)
		commands.CreateSubnetEvmConfig(subnetName, utils.SubnetEvmGenesisPath)
	})

	ginkgo.AfterEach(func() {
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})

	ginkgo.It("deploy subnet to fuji", func() {
		// deploy
		s := commands.SimulateDeploySubnetPublicly(subnetName, keyName, controlKeys)
		subnetID, rpcURL, err := utils.ParsePublicDeployOutput(s)
		gomega.Expect(err).Should(gomega.BeNil())
		// add validators to subnet
		nodeInfos, err := utils.GetNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		for _, nodeInfo := range nodeInfos {
			start := time.Now().Add(time.Second * 30).UTC().Format("2006-01-02 15:04:05")
			_ = commands.SimulateAddValidatorPublicly(subnetName, keyName, nodeInfo.ID, start, "24h", "20")
		}
		// join to copy vm binary and update config file
		for _, nodeInfo := range nodeInfos {
			_ = commands.SimulateJoinPublicly(subnetName, nodeInfo.ConfigFile, nodeInfo.PluginDir)
		}
		// get and check whitelisted subnets from config file
		var whitelistedSubnets string
		for _, nodeInfo := range nodeInfos {
			whitelistedSubnets, err = utils.GetWhilelistedSubnetsFromConfigFile(nodeInfo.ConfigFile)
			gomega.Expect(err).Should(gomega.BeNil())
			whitelistedSubnetsSlice := strings.Split(whitelistedSubnets, ",")
			gomega.Expect(whitelistedSubnetsSlice).Should(gomega.ContainElement(subnetID))
		}
		// update nodes whitelisted subnets
		err = utils.UpdateNodesWhitelistedSubnets(whitelistedSubnets)
		gomega.Expect(err).Should(gomega.BeNil())
		// wait for subnet walidators to be up
		err = utils.WaitSubnetValidators(subnetID, nodeInfos)
		gomega.Expect(err).Should(gomega.BeNil())
		// hardhat
		err = utils.SetHardhatRPC(rpcURL)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// this is a simulation, so app is probably saving the info in the
		// `local network` section of the sidecar instead of the `fuji` section...
		// ...need to manipulate the `fuji` section of the sidecar to contain the subnetID info
		// so that the `stats` command for `fuji` can find it
		output := commands.SimulateGetSubnetStatsFuji(subnetName, subnetID)
		gomega.Expect(output).Should(gomega.Not(gomega.BeNil()))
		gomega.Expect(output).Should(gomega.ContainSubstring("Current validators"))
		gomega.Expect(output).Should(gomega.ContainSubstring("NodeID-"))
		gomega.Expect(output).Should(gomega.ContainSubstring("No pending validators found"))
	})
})

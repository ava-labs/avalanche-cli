// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
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
		//_ = commands.StartNetwork()
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
		//commands.CleanNetwork()
	})

	ginkgo.It("deploy subnet to fuji", func() {
		// deploy
		s := commands.SimulateFujiDeploy(subnetName, keyName, controlKeys)
		subnetID, rpcURL, err := utils.ParsePublicDeployOutput(s)
		gomega.Expect(err).Should(gomega.BeNil())
		// add validators to subnet
		nodeInfos, err := utils.GetNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		for _, nodeInfo := range nodeInfos {
			start := time.Now().Add(time.Second * 30).UTC().Format("2006-01-02 15:04:05")
			_ = commands.SimulateFujiAddValidator(subnetName, keyName, nodeInfo.ID, start, "24h", "20")
		}
		// join to copy vm binary and update config file
		for _, nodeInfo := range nodeInfos {
			_ = commands.SimulateFujiJoin(subnetName, nodeInfo.ConfigFile, nodeInfo.PluginDir, nodeInfo.ID)
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
	})

	ginkgo.It("deploy subnet to mainnet", ginkgo.Label("local_machine"), func() {
		// fund ledger address
		err := utils.FundLedgerAddress()
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(logging.LightRed.Wrap("DEPLOYING SUBNET. VERIFY LEDGER ADDRESS HAS CUSTOM HRP BEFORE SIGNING"))
		s := commands.SimulateMainnetDeploy(subnetName)
		// deploy
		subnetID, rpcURL, err := utils.ParsePublicDeployOutput(s)
		gomega.Expect(err).Should(gomega.BeNil())
		// add validators to subnet
		nodeInfos, err := utils.GetNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		for i, nodeInfo := range nodeInfos {
			fmt.Println(logging.LightRed.Wrap(
				fmt.Sprintf("ADDING VALIDATOR %d of %d. VERIFY LEDGER ADDRESS HAS CUSTOM HRP BEFORE SIGNING", i, len(nodeInfos))))
			start := time.Now().Add(time.Second * 30).UTC().Format("2006-01-02 15:04:05")
			_ = commands.SimulateMainnetAddValidator(subnetName, nodeInfo.ID, start, "24h", "20")
		}
		fmt.Println(logging.LightBlue.Wrap("EXECUTING NON INTERACTIVE PART OF THE TEST: JOIN/WHITELIST/WAIT/HARDHAT"))
		// join to copy vm binary and update config file
		for _, nodeInfo := range nodeInfos {
			_ = commands.SimulateMainnetJoin(subnetName, nodeInfo.ConfigFile, nodeInfo.PluginDir, nodeInfo.ID)
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
	})
})

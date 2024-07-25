// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package packageman

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName       = "e2eSubnetTest"
	secondSubnetName = "e2eSecondSubnetTest"
)

var (
	binaryToVersion map[string]string
	err             error
)

var _ = ginkgo.Describe("[Package Management]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeAll(func() {
		mapper := utils.NewVersionMapper()
		binaryToVersion, err = utils.GetVersionMapping(mapper)
		gomega.Expect(err).Should(gomega.BeNil())
	})
	ginkgo.BeforeEach(func() {
		commands.CleanNetworkHard()
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can deploy a subnet with subnet-evm version", func() {
		// check subnet-evm install precondition
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey1])).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.SoloAvagoKey])).Should(gomega.BeFalse())

		commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, binaryToVersion[utils.SoloSubnetEVMKey1])
		deployOutput := commands.DeploySubnetLocallyWithVersion(subnetName, binaryToVersion[utils.SoloAvagoKey])
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// check subnet-evm install
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey1])).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.SoloAvagoKey])).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy multiple subnet-evm versions", func() {
		// check subnet-evm install precondition
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey1])).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey2])).Should(gomega.BeFalse())

		commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, binaryToVersion[utils.SoloSubnetEVMKey1])
		commands.CreateSubnetEvmConfigWithVersion(secondSubnetName, utils.SubnetEvmGenesis2Path, binaryToVersion[utils.SoloSubnetEVMKey2])

		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpcs1, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs1).Should(gomega.HaveLen(1))

		deployOutput = commands.DeploySubnetLocally(secondSubnetName)
		rpcs2, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs2).Should(gomega.HaveLen(1))

		err = utils.SetHardhatRPC(rpcs1[0])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.SetHardhatRPC(rpcs2[0])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// check subnet-evm install
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey1])).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckSubnetEVMExists(binaryToVersion[utils.SoloSubnetEVMKey2])).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})

	ginkgo.It("can deploy with multiple avalanchego versions", func() {
		// check avago install precondition
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago1Key])).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago2Key])).Should(gomega.BeFalse())

		commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, binaryToVersion[utils.MultiAvagoSubnetEVMKey])
		deployOutput := commands.DeploySubnetLocallyWithVersion(subnetName, binaryToVersion[utils.MultiAvago1Key])
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		// Deploy greeter contract
		scriptOutput, scriptErr, err := utils.RunHardhatScript(utils.GreeterScript)
		if scriptErr != "" {
			fmt.Println(scriptOutput)
			fmt.Println(scriptErr)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.ParseGreeterAddress(scriptOutput)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// check avago install
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago1Key])).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago2Key])).Should(gomega.BeFalse())

		commands.CleanNetwork()

		deployOutput = commands.DeploySubnetLocallyWithVersion(subnetName, binaryToVersion[utils.MultiAvago2Key])
		rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc = rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// check avago install
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago1Key])).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(binaryToVersion[utils.MultiAvago2Key])).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
	})
})

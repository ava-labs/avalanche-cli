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
	genesisPath      = "tests/e2e/assets/test_genesis.json"

	subnetEVMVersion1 = "v0.2.8"
	subnetEVMVersion2 = "v0.2.7"
	avagoVersion1     = "v1.7.16"
	avagoVersion2     = "v1.7.15"
)

var _ = ginkgo.Describe("[Package Management]", func() {
	ginkgo.BeforeEach(func() {
		err := utils.DeleteBins()
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteBins()
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can deploy a subnet with subnet-evm version", func() {
		// check subnet-evm install precondition
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion1)).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion1)).Should(gomega.BeFalse())

		commands.CreateSubnetConfigWithVersion(subnetName, genesisPath, subnetEVMVersion1)
		deployOutput := commands.DeploySubnetLocallyWithVersion(subnetName, avagoVersion1)
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
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion1)).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion1)).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy multiple subnet-evm versions", func() {
		// check subnet-evm install precondition
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion1)).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion2)).Should(gomega.BeFalse())

		commands.CreateSubnetConfigWithVersion(subnetName, genesisPath, subnetEVMVersion1)
		commands.CreateSubnetConfigWithVersion(secondSubnetName, genesisPath, subnetEVMVersion2)

		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		deployOutput = commands.DeploySubnetLocally(secondSubnetName)
		rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(2))

		err = utils.SetHardhatRPC(rpcs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.SetHardhatRPC(rpcs[1])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		// check subnet-evm install
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion1)).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckSubnetEVMExists(subnetEVMVersion2)).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})

	ginkgo.It("can deploy with multiple avalanchego versions", func() {
		// check avago install precondition
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion1)).Should(gomega.BeFalse())
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion2)).Should(gomega.BeFalse())

		commands.CreateSubnetConfig(subnetName, genesisPath)
		deployOutput := commands.DeploySubnetLocallyWithVersion(subnetName, avagoVersion1)
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
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion1)).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion2)).Should(gomega.BeFalse())

		commands.CleanNetwork()

		deployOutput = commands.DeploySubnetLocallyWithVersion(subnetName, avagoVersion2)
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
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion1)).Should(gomega.BeTrue())
		gomega.Expect(utils.CheckAvalancheGoExists(avagoVersion2)).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
	})
})

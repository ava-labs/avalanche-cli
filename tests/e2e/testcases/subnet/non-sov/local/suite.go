// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName       = "e2eSubnetTest"
	secondSubnetName = "e2eSecondSubnetTest"
	confPath         = "tests/e2e/assets/test_avalanche-cli.json"
)

var (
	mapping map[string]string
	err     error
)

var _ = ginkgo.Describe("[Local Subnet non SOV]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeAll(func() {
		mapper := utils.NewVersionMapper()
		mapping, err = utils.GetVersionMapping(mapper)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		_, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.DeleteConfigs(subnetName)
		if err != nil {
			fmt.Println("Clean network error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		if err != nil {
			fmt.Println("Delete config error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		// delete custom vm
		utils.DeleteCustomBinary(subnetName)
		utils.DeleteCustomBinary(secondSubnetName)
	})

	ginkgo.It("can deploy a custom vm subnet to local non SOV", func() {
		customVMPath, err := utils.DownloadCustomVMBin(mapping[utils.SoloSubnetEVMKey1])
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CreateCustomVMConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, customVMPath)
		deployOutput := commands.DeploySubnetLocallyWithVersionNonSOV(subnetName, mapping[utils.SoloAvagoKey])
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

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy a SubnetEvm subnet to local non SOV", func() {
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
		deployOutput := commands.DeploySubnetLocallyNonSOV(subnetName)
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

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can load viper config and setup node properties for local deploy non SOV", func() {
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
		deployOutput := commands.DeploySubnetLocallyWithViperConfNonSOV(subnetName, confPath)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]
		gomega.Expect(rpc).Should(gomega.HavePrefix("http://127.0.0.1:"))

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can't deploy the same subnet twice to local non SOV", func() {
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)

		deployOutput := commands.DeploySubnetLocallyNonSOV(subnetName)
		fmt.Println(deployOutput)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		out, err := commands.DeploySubnetLocallyWithArgsAndOutputNonSOV(subnetName, "", "")
		gomega.Expect(err).Should(gomega.HaveOccurred())
		deployOutput = string(out)
		rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
		if err == nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(rpcs).Should(gomega.HaveLen(0))
		gomega.Expect(deployOutput).Should(gomega.ContainSubstring("has already been deployed"))
	})

	ginkgo.It("can deploy multiple subnets to local non SOV", func() {
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
		commands.CreateSubnetEvmConfigNonSOV(secondSubnetName, utils.SubnetEvmGenesis2Path, false)

		deployOutput := commands.DeploySubnetLocallyNonSOV(subnetName)
		rpcs1, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs1).Should(gomega.HaveLen(1))

		deployOutput = commands.DeploySubnetLocallyNonSOV(secondSubnetName)
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

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})
})

var _ = ginkgo.Describe("[Subnet Compatibility]", func() {
	ginkgo.AfterEach(func() {
		_, _ = commands.CleanNetwork()
		if err := utils.DeleteConfigs(subnetName); err != nil {
			fmt.Println("Clean network error:", err)
			gomega.Expect(err).Should(gomega.BeNil())
		}

		if err := utils.DeleteConfigs(secondSubnetName); err != nil {
			fmt.Println("Delete config error:", err)
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("can deploy a subnet-evm with old version non SOV", func() {
		subnetEVMVersion := "v0.7.1"

		commands.CreateSubnetEvmConfigWithVersionNonSOV(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion, false)
		deployOutput := commands.DeploySubnetLocallyNonSOV(subnetName)
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

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can't deploy conflicting vm versions non SOV", func() {
		// TODO: These shouldn't be hardcoded either
		subnetEVMVersion1 := "v0.7.1"
		subnetEVMVersion2 := "v0.7.0"

		commands.CreateSubnetEvmConfigWithVersionNonSOV(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion1, false)
		commands.CreateSubnetEvmConfigWithVersionNonSOV(secondSubnetName, utils.SubnetEvmGenesis2Path, subnetEVMVersion2, false)

		deployOutput := commands.DeploySubnetLocallyNonSOV(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		commands.DeploySubnetLocallyExpectErrorNonSOV(secondSubnetName)

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})
})

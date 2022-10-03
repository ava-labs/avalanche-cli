// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apm

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	anr_utils "github.com/ava-labs/avalanche-network-runner/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName       = "e2eSubnetTest"
	secondSubnetName = "e2eSecondSubnetTest"

	subnetEVMVersion1 = "v0.3.0"
	subnetEVMVersion2 = "v0.2.9"
	// avagoVersion      = "v1.7.18"

	controlKeys = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	keyName     = "ewoq"

	subnetEVM029Hash = "9e8c1063b2965db21d2a759f0caedfca09d1e3f45edd2759c418acec22477a93"
	subnetEVM030Hash = "83c2ef4e478c37be6d2191da0b1b47f9bd449ac06c3e6ca7d04a274c47bccb0e"
)

var _ = ginkgo.Describe("[Upgrade]", func() {
	ginkgo.BeforeEach(func() {
		commands.CleanNetworkHard()
		// commands.CleanNetwork()
		// local network
		// _ = commands.StartNetworkWithVersion(avagoVersion)
		_ = commands.StartNetwork()
		// key
		_ = utils.DeleteKey(keyName)
		output, err := commands.CreateKeyFromPath(keyName, utils.EwoqKeyPath)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	// ginkgo.It("can create and update future", func() {
	// 	commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion1)

	// 	// check version
	// 	output, err := commands.DescribeSubnet(subnetName)
	// 	gomega.Expect(err).Should(gomega.BeNil())

	// 	containsVersion1 := strings.Contains(output, subnetEVMVersion1)
	// 	containsVersion2 := strings.Contains(output, subnetEVMVersion2)
	// 	gomega.Expect(containsVersion1).Should(gomega.BeTrue())
	// 	gomega.Expect(containsVersion2).Should(gomega.BeFalse())

	// 	output, err = commands.UpgradeVMFuture(subnetName, subnetEVMVersion2)
	// 	gomega.Expect(err).Should(gomega.BeNil())
	// 	if err != nil {
	// 		fmt.Println(output)
	// 	}

	// 	output, err = commands.DescribeSubnet(subnetName)
	// 	gomega.Expect(err).Should(gomega.BeNil())

	// 	containsVersion1 = strings.Contains(output, subnetEVMVersion1)
	// 	containsVersion2 = strings.Contains(output, subnetEVMVersion2)
	// 	gomega.Expect(containsVersion1).Should(gomega.BeFalse())
	// 	gomega.Expect(containsVersion2).Should(gomega.BeTrue())

	// 	commands.DeleteSubnetConfig(subnetName)
	// })

	ginkgo.It("can upgrade subnet-evm on public deployment", func() {
		// sub29Hash, err := utils.GetFileHash("/Users/connor/.avalanche-cli/bin/subnet-evm/subnet-evm-v0.2.9/subnet-evm")
		// gomega.Expect(err).Should(gomega.BeNil())
		// sub30Hash, err := utils.GetFileHash("/Users/connor/.avalanche-cli/bin/subnet-evm/subnet-evm-v0.3.0/subnet-evm")
		// gomega.Expect(err).Should(gomega.BeNil())

		// fmt.Println("2.9 Hash:", sub29Hash)
		// fmt.Println("3.0 Hash:", sub30Hash)

		commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion1)

		// Simulate fuji deployment
		s := commands.SimulateDeploySubnetPublicly(subnetName, keyName, controlKeys)
		subnetID, _, err := utils.ParsePublicDeployOutput(s)
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
		err = utils.RestartNodesWithWhitelistedSubnets(whitelistedSubnets)
		gomega.Expect(err).Should(gomega.BeNil())
		// wait for subnet walidators to be up
		err = utils.WaitSubnetValidators(subnetID, nodeInfos)
		gomega.Expect(err).Should(gomega.BeNil())

		// upgrade the vm on each node
		vmid, err := anr_utils.VMID(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		first := true
		for _, nodeInfo := range nodeInfos {
			// check the current node version
			vmVersion, err := utils.GetNodeVMVersion(nodeInfo.URI, vmid.String())
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(vmVersion).Should(gomega.Equal(subnetEVMVersion1))

			if first {
				originalHash, err := utils.GetFileHash(filepath.Join(nodeInfo.PluginDir, vmid.String()))
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(originalHash).Should(gomega.Equal(subnetEVM030Hash))
				first = false
			}

			output, err := commands.UpgradeVMPublic(subnetName, subnetEVMVersion2, nodeInfo.PluginDir)
			gomega.Expect(err).Should(gomega.BeNil())
			fmt.Println(output)
			if err != nil {
				fmt.Println(output)
			}
		}

		// TODO: There is currently only one subnet-evm version compatible with avalanchego. These
		// lines should be uncommented when a new version is released. The section below can be removed.
		// // restart to use the new vm version
		// err = utils.RestartNodesWithWhitelistedSubnets(whitelistedSubnets)
		// gomega.Expect(err).Should(gomega.BeNil())
		// // wait for subnet walidators to be up
		// err = utils.WaitSubnetValidators(subnetID, nodeInfos)
		// gomega.Expect(err).Should(gomega.BeNil())

		// // Check that nodes are running the new version
		// for _, nodeInfo := range nodeInfos {
		// 	// check the current node version
		// 	vmVersion, err := utils.GetNodeVMVersion(nodeInfo.URI, vmid.String())
		// 	gomega.Expect(err).Should(gomega.BeNil())
		// 	gomega.Expect(vmVersion).Should(gomega.Equal(subnetEVMVersion2))
		// }

		// This can be removed when the above is added
		for _, nodeInfo := range nodeInfos {
			measuredHash, err := utils.GetFileHash(filepath.Join(nodeInfo.PluginDir, vmid.String()))
			gomega.Expect(err).Should(gomega.BeNil())

			gomega.Expect(measuredHash).Should(gomega.Equal(subnetEVM029Hash))
		}

		// Stop removal here
		////////////////////////////////////////////////////////

		commands.DeleteSubnetConfig(subnetName)
	})

	// ginkgo.It("can deploy a subnet with subnet-evm version", func() {
	// 	commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion1)
	// 	deployOutput := commands.DeploySubnetLocallyWithVersion(subnetName, avagoVersion)
	// 	rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
	// 	if err != nil {
	// 		fmt.Println(deployOutput)
	// 	}
	// 	gomega.Expect(err).Should(gomega.BeNil())
	// 	gomega.Expect(rpcs).Should(gomega.HaveLen(1))
	// 	rpc := rpcs[0]

	// 	err = utils.SetHardhatRPC(rpc)
	// 	gomega.Expect(err).Should(gomega.BeNil())

	// 	err = utils.RunHardhatTests(utils.BaseTest)
	// 	gomega.Expect(err).Should(gomega.BeNil())

	// 	commands.DeleteSubnetConfig(subnetName)
	// })
})

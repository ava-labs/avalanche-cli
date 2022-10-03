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

	controlKeys = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	keyName     = "ewoq"
)

var _ = ginkgo.Describe("[Upgrade]", func() {
	ginkgo.BeforeEach(func() {
		commands.CleanNetworkHard()
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
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can create and update future", func() {
		commands.CreateSubnetEvmConfigWithVersion(subnetName, utils.SubnetEvmGenesisPath, subnetEVMVersion1)

		// check version
		output, err := commands.DescribeSubnet(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())

		containsVersion1 := strings.Contains(output, subnetEVMVersion1)
		containsVersion2 := strings.Contains(output, subnetEVMVersion2)
		gomega.Expect(containsVersion1).Should(gomega.BeTrue())
		gomega.Expect(containsVersion2).Should(gomega.BeFalse())

		output, err = commands.UpgradeVMFuture(subnetName, subnetEVMVersion2)
		gomega.Expect(err).Should(gomega.BeNil())
		if err != nil {
			fmt.Println(output)
		}

		output, err = commands.DescribeSubnet(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())

		containsVersion1 = strings.Contains(output, subnetEVMVersion1)
		containsVersion2 = strings.Contains(output, subnetEVMVersion2)
		gomega.Expect(containsVersion1).Should(gomega.BeFalse())
		gomega.Expect(containsVersion2).Should(gomega.BeTrue())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can upgrade subnet-evm on public deployment", func() {
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

		// TODO Delete this after updating
		var originalHash string

		// upgrade the vm on each node
		vmid, err := anr_utils.VMID(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())

		for _, nodeInfo := range nodeInfos {
			// check the current node version
			vmVersion, err := utils.GetNodeVMVersion(nodeInfo.URI, vmid.String())
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(vmVersion).Should(gomega.Equal(subnetEVMVersion1))

			originalHash, err = utils.GetFileHash(filepath.Join(nodeInfo.PluginDir, vmid.String()))
			gomega.Expect(err).Should(gomega.BeNil())
		}

		// stop network
		commands.StopNetwork()

		for _, nodeInfo := range nodeInfos {
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

			gomega.Expect(measuredHash).ShouldNot(gomega.Equal(originalHash))
		}

		// Stop removal here
		////////////////////////////////////////////////////////

		commands.DeleteSubnetConfig(subnetName)
	})
})

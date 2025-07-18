// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanche-cli/cmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Start", ginkgo.Ordered, func() {
	ginkgo.AfterEach(func() {
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("can start network with default params", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", constants.LocalNetworkNumNodes)))
		gomega.Expect(out).Should(gomega.ContainSubstring("Network Healthy: true"))
		gomega.Expect(out).Should(gomega.ContainSubstring("Blockchains Healthy: true"))
	})

	ginkgo.It("can start network with given number of nodes", func() {
		numOfNodes := uint(3)
		out, err := commands.StartNetworkWithParams(map[string]interface{}{
			"num-nodes": strconv.FormatUint(uint64(numOfNodes), 10),
		})
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", numOfNodes)))
		gomega.Expect(out).Should(gomega.ContainSubstring("Network Healthy: true"))
		gomega.Expect(out).Should(gomega.ContainSubstring("Blockchains Healthy: true"))
	})

	ginkgo.It("should not start network with already started network", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		out, err = commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network has already been booted"))

		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
	})

	ginkgo.It("can start stopped network with preserved state - default snapshot", func() {
		// start network with given number of nodes
		numOfNodes := uint(5)
		out, err := commands.StartNetworkWithParams(map[string]interface{}{
			"num-nodes": strconv.FormatUint(uint64(numOfNodes), 10),
		})
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status - number of nodes
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", numOfNodes)))

		// stop the network
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())

		// now start the network again with no arguments
		out, err = commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status - number of nodes should be preserved
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", numOfNodes)))
	})

	ginkgo.It("can start properly with given snapshot - custom snapshot", func() {
		// start network with given number of nodes
		numOfNodes := uint(5)
		out, err := commands.StartNetworkWithParams(map[string]interface{}{
			"num-nodes": strconv.FormatUint(uint64(numOfNodes), 10),
		})
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// stop with snapshot name
		testSnapshotName := "test-snapshot"
		err = commands.StopNetwork("--snapshot-name", testSnapshotName)
		gomega.Expect(err).Should(gomega.BeNil())

		// check snapshot exists
		snapshotExists := utils.CheckSnapshotExists(testSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", testSnapshotName))

		// start the network with snapshot name
		out, err = commands.StartNetworkWithParams(map[string]interface{}{
			"snapshot-name": testSnapshotName,
		})
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status - number of nodes should be preserved
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", numOfNodes)))

		utils.DeleteSnapshot(testSnapshotName)
	})

	ginkgo.It("can start a deployed but stopped L1", func() {
		testSubnetName := "testSubnet1"
		commands.CreateEtnaSubnetEvmConfig(testSubnetName, utils.EwoqEVMAddress, commands.PoA)

		// Deploy a local L1
		out, err := commands.DeployBlockchain(
			testSubnetName,
			utils.TestFlags{
				"local":             true,
				"skip-icm-deploy":   true,
				"skip-update-check": true,
			},
		)
		gomega.Expect(out).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		// stop the network
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())

		// now restart the network
		out, err = commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check L1 status - should be back and running
		out, err = utils.TestCommand(
			cmd.BlockchainCmd,
			"stats",
			[]string{
				testSubnetName,
				"--local",
				"--" + constants.SkipUpdateFlag,
			},
			utils.GlobalFlags{},
			utils.TestFlags{},
		)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("already validating the subnet"))

		commands.DeleteSubnetConfig(testSubnetName)
	})

	ginkgo.It("can start with given avalanchego version", func() {
		testSubnetName := "testSubnet3"
		// subnet config
		_, avagoVersion := commands.CreateSubnetEvmConfigSOV(testSubnetName, utils.SubnetEvmGenesisPath)

		// local network
		_, err := commands.StartNetworkWithParams(map[string]interface{}{
			"avalanchego-version": avagoVersion,
		})
		gomega.Expect(err).Should(gomega.BeNil())

		_ = utils.DeleteConfigs(testSubnetName)
	})
})

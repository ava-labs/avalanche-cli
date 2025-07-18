// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Clean", ginkgo.Ordered, func() {
	ginkgo.It("can clean a started network", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		out, err = commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Process terminated"))
	})

	ginkgo.It("can clean a deployed L1", func() {
		testSubnetName := "testSubnet2"
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

		// Clean the network
		out, err = commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Process terminated"))

		// Check L1 status - should not be running
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
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("no subnetID found for the provided blockchain name"))

		commands.DeleteSubnetConfig(testSubnetName)
	})

	ginkgo.It("should err out when no network is running", func() {
		out, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("No network is running"))
	})

	ginkgo.It("should only clean default snapshot", func() {
		// Start the network
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// Stop the network with default snapshot
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())

		// check if default snapshot exists
		snapshotExists := utils.CheckSnapshotExists("default")
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", "default"))

		// Start the network
		out, err = commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// Stop the network with custome snapshot
		testSnapshotName := "test-snapshot"
		err = commands.StopNetwork("--snapshot-name", testSnapshotName)
		gomega.Expect(err).Should(gomega.BeNil())

		// check if custom snapshot exists
		snapshotExists = utils.CheckSnapshotExists(testSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", testSnapshotName))

		// Clean the network
		out, err = commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("No network is running"))

		// default snapshot should be cleaned up
		snapshotExists = utils.CheckSnapshotExists("default")
		gomega.Expect(snapshotExists).Should(gomega.BeFalse(),
			fmt.Sprintf("snapshot %s should not exist", "default"))

		// custom snapshot should still exist
		snapshotExists = utils.CheckSnapshotExists(testSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", testSnapshotName))

		utils.DeleteSnapshot(testSnapshotName)
	})
})

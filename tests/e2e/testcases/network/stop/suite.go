// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network]", ginkgo.Ordered, func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
	})

	ginkgo.It("can stop a started network", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		commands.StopNetwork()

		// check network status
		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// check snapshot exists
		snapshotExists := utils.CheckSnapshotExists(constants.DefaultSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(), "default snapshot should exist")

		// TODO
		// clean up the snapshot
		// utils.DeleteSnapshot(constants.DefaultSnapshotName)
	})

	ginkgo.It("can stop a started network with --dont-save", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		commands.StopNetwork("--dont-save")

		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// TODO
	})

	ginkgo.It("can stop a started network with --snapshot-name", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		testSnapshotName := "test-snapshot"
		commands.StopNetwork("--snapshot-name", testSnapshotName)

		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// check snapshot exists
		snapshotExists := utils.CheckSnapshotExists(testSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", testSnapshotName))

		// clean up the snapshot
		utils.DeleteSnapshot(testSnapshotName)
	})
})

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Stop", ginkgo.Ordered, func() {
	ginkgo.AfterEach(func() {
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("can stop a started network", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())

		// check network status
		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// check default snapshot exists
		snapshotExists := utils.CheckSnapshotExists(constants.DefaultSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(), "default snapshot should exist")

		// clean up the snapshot
		utils.DeleteSnapshot(constants.DefaultSnapshotName)
	})

	ginkgo.It("can stop a started network with --dont-save", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		err = commands.StopNetwork("--dont-save")
		gomega.Expect(err).Should(gomega.BeNil())

		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// make sure no snapshots exist
		entries, err := os.ReadDir(utils.GetSnapshotsDir())
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(entries).Should(gomega.BeEmpty(), "no snapshots should exist")
	})

	ginkgo.It("can stop a started network with --snapshot-name", func() {
		out, err := commands.StartNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		testSnapshotName := "test-snapshot"
		err = commands.StopNetwork("--snapshot-name", testSnapshotName)
		gomega.Expect(err).Should(gomega.BeNil())

		out, err = commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))

		// check snapshot exists
		snapshotExists := utils.CheckSnapshotExists(testSnapshotName)
		gomega.Expect(snapshotExists).Should(gomega.BeTrue(),
			fmt.Sprintf("snapshot %s should exist", testSnapshotName))

		// clean up the snapshot
		utils.DeleteSnapshot(testSnapshotName)
	})

	ginkgo.It("should fail when stop network when no network is up", func() {
		err := commands.StopNetwork()
		gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
	})
})

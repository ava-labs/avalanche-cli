// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package root

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Node local start]", func() {
	testClusterName := "test-cluster"

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
	})

	ginkgo.It("can create a local node with a started network", func() {
		output := commands.StartNetwork()
		gomega.Expect(output).Should(gomega.ContainSubstring("Network ready to use"))

		output, err := commands.NodeLocalStart(testClusterName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).Should(gomega.ContainSubstring("NodeID: NodeID-"))
	})

	ginkgo.It("should not create local nodes without a started network", func() {
		output, _ := commands.NodeLocalStart(testClusterName)
		gomega.Expect(output).Should(gomega.ContainSubstring("network is not running"))
	})
})

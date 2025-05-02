// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Start", ginkgo.Ordered, func() {
	ginkgo.AfterEach(func() {
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("can start network with default params", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// https://github.com/ava-labs/avalanchego/blob/master/tests/fixture/tmpnet/defaults.go#L27
		defaultNodeCount := 2

		// check network status
		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", defaultNodeCount)))
		gomega.Expect(out).Should(gomega.ContainSubstring("Network Healthy: true"))
		gomega.Expect(out).Should(gomega.ContainSubstring("Blockchains Healthy: true"))
	})

	ginkgo.It("can start network with given number of nodes", func() {
		numOfNodes := uint(3)
		out := commands.StartNetworkWithParams(map[string]string{
			"number-of-nodes": strconv.FormatUint(uint64(numOfNodes), 10),
		})
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		// check network status
		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
		gomega.Expect(out).Should(gomega.ContainSubstring(fmt.Sprintf("Number of Nodes: %d", numOfNodes)))
		gomega.Expect(out).Should(gomega.ContainSubstring("Network Healthy: true"))
		gomega.Expect(out).Should(gomega.ContainSubstring("Blockchains Healthy: true"))
	})

	ginkgo.It("should not start network with already started network", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		out = commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network has already been booted"))

		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Network is Up"))
	})
})

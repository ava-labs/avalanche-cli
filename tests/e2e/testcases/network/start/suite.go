// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
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
})

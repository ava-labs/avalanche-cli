// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Status", ginkgo.Ordered, func() {
	ginkgo.AfterEach(func() {
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("can get status of started network", func() {
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

	ginkgo.It("can get status when no network is up", func() {
		out, err := commands.GetNetworkStatus()
		gomega.Expect(err).ShouldNot(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("network is not running"))
	})
})

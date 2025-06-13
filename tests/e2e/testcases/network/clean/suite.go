// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
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

	ginkgo.It("should err out when no network is running", func() {
		out, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("No network is running"))
	})
})

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"path"

	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	testUtils "github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Local Network] Clean", ginkgo.Ordered, func() {
	ginkgo.It("can clean a started network", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		out, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("Process terminated"))
	})

	ginkgo.It("should err out when no network is running", func() {
		out, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(out).Should(gomega.ContainSubstring("No network is running"))
	})

	ginkgo.It("hard clean should remove downloaded avalanchego and plugins", func() {
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))

		_, err := commands.CleanNetworkHard()
		gomega.Expect(err).Should(gomega.BeNil())

		// check if binaries are removed
		binDirExists := utils.DirExists(path.Join(testUtils.GetBaseDir(), "bin"))
		gomega.Expect(binDirExists).Should(gomega.BeFalse())
	})
})

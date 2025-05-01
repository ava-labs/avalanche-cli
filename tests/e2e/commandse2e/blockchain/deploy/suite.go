// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package deploy

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName = "testSubnet"
)

const ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

var _ = ginkgo.Describe("[Blockchain Deploy Flags]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeEach(func() {
		// Create test subnet config
		commands.CreateEtnaSubnetEvmConfig(subnetName, ewoqEVMAddress, commands.PoA)
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		// Cleanup test subnet config
		commands.DeleteSubnetConfig(subnetName)
	})
	blockchainCmdArgs := []string{subnetName}
	globalFlags := utils.GlobalFlags{
		"local":             true,
		"skip-icm-deploy":   true,
		"skip-update-check": true,
	}
	ginkgo.It("HAPPY PATH: local deploy default", func() {
		testFlags := utils.TestFlags{}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("ERROR PATH: invalid_version", func() {
		testFlags := utils.TestFlags{
			"avalanchego-version": "invalid_version",
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	})
})

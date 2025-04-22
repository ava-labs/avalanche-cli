// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package deploy

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commandse2e"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	deployTestJSONPath = "tests/e2e/commandse2e/blockchain/deploy/deploy_tests.json"
	subnetName         = "testSubnet"
)

var (
	config *commandse2e.TestJSONConfig
	err    error
)

const ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

var _ = ginkgo.Describe("[Blockchain Deploy Flags]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeEach(func() {
		// Create test subnet config
		commands.CreateSubnetEvmConfigSOV(subnetName, ewoqEVMAddress, commands.PoA)

		// Read test configuration
		config, err = commandse2e.ReadTestConfig(deployTestJSONPath)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		// Cleanup test subnet config
		commands.DeleteSubnetConfig(subnetName)
	})
	blockchainCmdArgs := []string{subnetName}
	ginkgo.It("should successfully deploy a blockchain", func() {
		// Run each happy path test case
		for _, testCase := range config.HappyPath {
			ginkgo.By(fmt.Sprintf("Running test case: %s", testCase.Name))
			output, err := commandse2e.TestCommandWithJSONConfig(
				commandse2e.BlockchainCmd,
				"deploy",
				blockchainCmdArgs,
				deployTestJSONPath,
				&testCase,
			)
			//if testCase.ExpectedOutput != "" {
			//	gomega.Expect(output).Should(gomega.ContainSubstring(testCase.ExpectedOutput))
			//}
			if len(testCase.ExpectedOutput) > 0 {
				for _, expectedOutput := range testCase.ExpectedOutput {
					gomega.Expect(output).Should(gomega.ContainSubstring(expectedOutput))
				}
			}
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("should handle error cases", func() {
		// Run each not happy path test case
		for _, testCase := range config.NotHappyPath {
			ginkgo.By(fmt.Sprintf("Running test case: %s", testCase.Name))
			_, err = commandse2e.TestCommandWithJSONConfig(
				commandse2e.BlockchainCmd,
				"deploy",
				blockchainCmdArgs,
				deployTestJSONPath,
				&testCase,
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
		}
	})
})

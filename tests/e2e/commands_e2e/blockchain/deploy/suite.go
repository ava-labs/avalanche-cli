// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package deploy

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands_e2e/blockchain"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	deployTestJsonPath = "tests/e2e/commands_e2e/blockchain/deploy/deploy_tests.json"
	subnetName         = "testSubnet"
)

var (
	config *blockchain.TestJSONConfig
	err    error
)

const ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

var _ = ginkgo.Describe("[Blockchain Deploy Flags]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeEach(func() {
		// Create test subnet config
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPath, ewoqEVMAddress)

		// Read test configuration
		config, err = blockchain.ReadTestConfig(deployTestJsonPath)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		// Cleanup test subnet config
		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("should successfully deploy a blockchain", func() {
		// Run each happy path test case
		for _, testCase := range config.HappyPath {
			ginkgo.By(fmt.Sprintf("Running test case: %s", testCase.Name))
			_, err = blockchain.TestCommandWithJSONConfig(
				"deploy",
				deployTestJsonPath,
				&testCase,
			)
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})
	//
	//ginkgo.It("should handle invalid configurations", func() {
	//	// Run each not happy path test case
	//	for _, testCase := range config.NotHappyPath {
	//		ginkgo.By(fmt.Sprintf("Running test case: %s", testCase.Name))
	//		_, err = blockchain.TestCommandWithJSONConfig(
	//			"deploy",
	//			deployTestJsonPath,
	//			&testCase,
	//		)
	//		gomega.Expect(err).Should(gomega.HaveOccurred())
	//	}
	//})
})

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package deploy

import (
	"fmt"
	"runtime"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
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
	//ginkgo.It("HAPPY PATH: local deploy default", func() {
	//	testFlags := utils.TestFlags{}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	//	gomega.Expect(err).Should(gomega.BeNil())
	//})
	ginkgo.It("HAPPY PATH: local deploy with avalanchego path set", func() {
		avalanchegoPath := "tests/e2e/assets/mac/avalanchego"
		if runtime.GOOS == "linux" {
			avalanchegoPath = "tests/e2e/assets/linux/avalanchego"
		}
		testFlags := utils.TestFlags{
			"avalanchego-path": avalanchegoPath,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("AvalancheGo path: %s", avalanchegoPath)))
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy convert only", func() {
		testFlags := utils.TestFlags{
			"convert-only": true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("To finish conversion to sovereign L1, create the corresponding Avalanche node(s) with the provided Node ID and BLS Info"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Ensure that the P2P port is exposed and 'public-ip' config value is set"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Once the Avalanche Node(s) are created and are tracking the blockchain, call `avalanche contract initValidatorManager testSubnet` to finish conversion to sovereign L1"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap validator balance", func() {
		testFlags := utils.TestFlags{
			"balance": 0.2,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	//ginkgo.It("HAPPY PATH: local deploy with bootstrap endpoints", func() {
	//	testFlags := utils.TestFlags{
	//		"bootstrap-endpoints": []string{"http://localhost:9650", "http://localhost:9651"},
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	//	gomega.Expect(err).Should(gomega.BeNil())
	//})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap filepath", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": "tests/e2e/assets/bootstrap.json",
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy with change owner address", func() {
		testFlags := utils.TestFlags{
			"change-owner-address": ewoqEVMAddress,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	// TODO: uncomment below when num-bootstrap-validators fix PR is merged
	//ginkgo.It("HAPPY PATH: local deploy set num bootstrap validators", func() {
	//	testFlags := utils.TestFlags{
	//		"num-bootstrap-validators": 2,
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	//	gomega.Expect(err).Should(gomega.BeNil())
	//})

	//ginkgo.It("ERROR PATH: invalid_version", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	ginkgo.It("ERROR PATH: invalid_avalanchego_path", func() {
		avalancheGoPath := "invalid_avalanchego_path"
		testFlags := utils.TestFlags{
			"avalanchego-path": avalancheGoPath,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("avalancheGo binary %s does not exist", avalancheGoPath)))
	})

	ginkgo.It("ERROR PATH: invalid balance value", func() {
		testFlags := utils.TestFlags{
			"balance": -1.0,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("balance must be positive"))
	})

	//ginkgo.It("ERROR PATH: invalid bootstrap endpoints", func() {
	//	testFlags := utils.TestFlags{
	//		"bootstrap-endpoints": []string{"invalid-url"},
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid bootstrap endpoint"))
	//})

	ginkgo.It("ERROR PATH: invalid bootstrap filepath", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": "nonexistent.json",
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("bootstrap file does not exist"))
	})

	ginkgo.It("ERROR PATH: invalid change owner address", func() {
		testFlags := utils.TestFlags{
			"change-owner-address": "invalid-address",
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("invalid change owner address"))
	})

	ginkgo.It("ERROR PATH: generate node id is not applicable for local network", func() {
		testFlags := utils.TestFlags{
			"generate-node-id": true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("number of bootstrap validators must be positive"))
	})

	ginkgo.It("ERROR PATH: generate node id is not applicable for local network", func() {
		testFlags := utils.TestFlags{
			"generate-node-id": true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("number of bootstrap validators must be positive"))
	})
})

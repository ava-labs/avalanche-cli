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

func checkConvertOnlyOutput(output string, generateNodeID bool) {
	gomega.Expect(output).Should(gomega.ContainSubstring("Converted blockchain successfully generated"))
	gomega.Expect(output).Should(gomega.ContainSubstring("Have the Avalanche node(s) track the blockchain"))
	gomega.Expect(output).Should(gomega.ContainSubstring("Call `avalanche contract initValidatorManager testSubnet`"))
	gomega.Expect(output).Should(gomega.ContainSubstring("Ensure that the P2P port is exposed and 'public-ip' config value is set"))
	gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	if generateNodeID {
		gomega.Expect(output).Should(gomega.ContainSubstring("Create the corresponding Avalanche node(s) with the provided Node ID and BLS Info"))
	} else {
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("Create the corresponding Avalanche node(s) with the provided Node ID and BLS Info"))
	}
}

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
		checkConvertOnlyOutput(output, false)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: generate node id ends in convert only", func() {
		testFlags := utils.TestFlags{
			"generate-node-id":         true,
			"num-bootstrap-validators": 1,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		checkConvertOnlyOutput(output, true)
		gomega.Expect(err).Should(gomega.BeNil())
		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
		gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(1))
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].NodeID).ShouldNot(gomega.BeNil())
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSProofOfPossession).ShouldNot(gomega.BeNil())
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSPublicKey).ShouldNot(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap validator balance", func() {
		testFlags := utils.TestFlags{
			"balance": 0.2,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		testFlags = utils.TestFlags{
			"--local":         true,
			"--validation-id": sc.Networks["Local Network"].BootstrapValidators[0].ValidationID,
		}
		output, err = utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, nil, testFlags)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.20000 AVAX"))
	})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap filepath", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath2,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		for i := 0; i < 2; i++ {
			testFlags := utils.TestFlags{
				"--local":         true,
				"--validation-id": sc.Networks["Local Network"].BootstrapValidators[i].ValidationID,
			}
			output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, nil, testFlags)
			gomega.Expect(err).Should(gomega.BeNil(), "Error for validator %d", i)
			if i == 0 {
				// we set first validator to have 0.2 AVAX balance in test_bootstrap_validator2.json
				gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.20000 AVAX"))
			} else {
				// we set second validator to have 0.3 AVAX balance in test_bootstrap_validator2.json
				gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.30000 AVAX"))
			}
		}
	})

	ginkgo.It("HAPPY PATH: local deploy with change owner address", func() {
		testFlags := utils.TestFlags{
			"change-owner-address": ewoqEVMAddress,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy set num bootstrap validators", func() {
		testFlags := utils.TestFlags{
			"num-bootstrap-validators": 2,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
		gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(2))

		//TODO: add local uri check

		//TODO: add poa type check
	})

	ginkgo.It("ERROR PATH: invalid_version", func() {
		testFlags := utils.TestFlags{
			"avalanchego-version": "invalid_version",
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	})

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

	ginkgo.It("ERROR PATH: generate node id is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"generate-node-id": true,
			"convert-only":     false,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --generate-node-id=true"))
	})
	ginkgo.It("ERROR PATH: generate node id is not applicable if use local machine is false", func() {
		testFlags := utils.TestFlags{
			"generate-node-id":  true,
			"use-local-machine": true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --generate-node-id=true"))
	})

	ginkgo.It("ERROR PATH: bootstrap filepath is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath,
			"convert-only":       false,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --bootstrap-filepath is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath is not applicable if use local machine is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath,
			"use-local-machine":  true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --bootstrap-filepath is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints": "127.0.0.1:9650",
			"convert-only":        false,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --bootstrap-endpoints is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if use local machine is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints": "127.0.0.1:9650",
			"use-local-machine":   true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --bootstrap-endpoints is not empty"))
	})
})

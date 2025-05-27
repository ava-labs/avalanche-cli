// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package addValidator

import (
	"fmt"
	utilsPkg "github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"os/exec"
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

var _ = ginkgo.Describe("[Blockchain Add Validator]", ginkgo.Ordered, func() {
	var nodeIDStr, publicKey, pop string
	_ = ginkgo.BeforeEach(func() {
		commands.StartNetwork()
		createValidatorCmd := exec.Command("./bin/avalanche", "node", "local", "start", "newNode", "--local")
		out, err := createValidatorCmd.CombinedOutput()
		fmt.Println(string(out))
		if err != nil {
			fmt.Printf("err %s \n", err.Error())
		}
		gomega.Expect(err).Should(gomega.BeNil())

		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Printf("localClusterUris %s \n", localClusterUris)
		//gomega.Expect(len(localClusterUris)).Should(gomega.Equal(1))

		fmt.Printf("localClusterUris[0] %s \n", localClusterUris[0])
		nodeIDStr, publicKey, pop, err = utilsPkg.GetNodeID(localClusterUris[0])
		gomega.Expect(err).Should(gomega.BeNil())

		fmt.Printf("nodeIDStr %s \n", nodeIDStr)
		fmt.Printf("publicKey %s \n", publicKey)
		fmt.Printf("pop %s \n", pop)

		// Create test subnet config
		commands.CreateEtnaSubnetEvmConfig(subnetName, ewoqEVMAddress, commands.PoA)
		globalFlags := utils.GlobalFlags{
			"local":             true,
			"skip-update-check": true,
		}
		blockchainCmdArgs := []string{subnetName}
		_, err = utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, nil)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	//ginkgo.AfterEach(func() {
	//	//app := utils.GetApp()
	//	//
	//	//createValidatorCmd := os.RemoveAll(app.GetBaseDir())
	//	_ = exec.Command("./bin/avalanche", "node", "local", "destroy", "newNode")
	//	_ = exec.Command("./bin/avalanche", "node", "local", "destroy", fmt.Sprintf("%s-local-node-local-network", subnetName))
	//	app := utils.GetApp()
	//	os.RemoveAll(filepath.Join(app.GetBaseDir(), "local"))
	commands.CleanNetwork()
	//	// Cleanup test subnet config
	//	commands.DeleteSubnetConfig(subnetName)
	//})
	blockchainCmdArgs := []string{subnetName}
	globalFlags := utils.GlobalFlags{
		"local":                   true,
		"ewoq":                    true,
		"weight":                  20,
		"skip-update-check":       true,
		"balance":                 0.1,
		"remaining-balance-owner": "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
		"disable-owner":           "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
	}
	//ginkgo.It("HAPPY PATH: add validator default", func() {
	//	fmt.Printf("HAPPYnodeIDStr %s \n", nodeIDStr)
	//	fmt.Printf("HAPPYpublicKey %s \n", publicKey)
	//	fmt.Printf("HAPPYpop %s \n", pop)
	//	testFlags := utils.TestFlags{
	//		"node-id":                 nodeIDStr,
	//		"bls-public-key":          publicKey,
	//		"bls-proof-of-possession": pop,
	//	}
	//	_, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
	//	if err != nil {
	//		fmt.Printf("err %s \n", err.Error())
	//	}
	//	gomega.Expect(err).Should(gomega.BeNil())
	//})
	//
	//ginkgo.It("HAPPY PATH: add validator with node endpoint", func() {
	//	avalanchegoPath := "tests/e2e/assets/mac/avalanchego"
	//	if runtime.GOOS == "linux" {
	//		avalanchegoPath = "tests/e2e/assets/linux/avalanchego"
	//	}
	//	testFlags := utils.TestFlags{
	//		"avalanchego-path": avalanchegoPath,
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("AvalancheGo path: %s", avalanchegoPath)))
	//	gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	//	gomega.Expect(err).Should(gomega.BeNil())
	//})
	//
	ginkgo.It("HAPPY PATH: add validator with create-local-validator", func() {
		testFlags := utils.TestFlags{
			"create-local-validator": true,
		}
		_, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		//checkConvertOnlyOutput(output, false)
		gomega.Expect(err).Should(gomega.BeNil())
		// we should have two local machine instances
		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(2))
	})

	//ginkgo.It("HAPPY PATH: add validator using rpc flag (remote L1)", func() {
	//	testFlags := utils.TestFlags{
	//		"generate-node-id":         true,
	//		"num-bootstrap-validators": 1,
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	checkConvertOnlyOutput(output, true)
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	sc, err := utils.GetSideCar(blockchainCmdArgs[0])
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
	//	gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(1))
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].NodeID).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSProofOfPossession).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSPublicKey).ShouldNot(gomega.BeNil())
	//})
	//ginkgo.It("HAPPY PATH: add validator with external signing group", func() {
	//	testFlags := utils.TestFlags{
	//		"generate-node-id":         true,
	//		"num-bootstrap-validators": 1,
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	checkConvertOnlyOutput(output, true)
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	sc, err := utils.GetSideCar(blockchainCmdArgs[0])
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
	//	gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(1))
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].NodeID).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSProofOfPossession).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSPublicKey).ShouldNot(gomega.BeNil())
	//})

	//ginkgo.It("ERROR PATH: add validator with incorrect validator manager owner address", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with too large weight", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with sov flags to a non sov blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//
	//ginkgo.It("ERROR PATH: add validator with non sov flags to an sov blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with pos flags to a poa blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with rpc flag with argument set", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with rpc flag to a non sov blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
})

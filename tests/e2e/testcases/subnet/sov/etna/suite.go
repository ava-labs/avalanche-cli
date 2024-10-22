// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	CLIBinary         = "./bin/avalanche"
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
	avalancheGoPath   = "--avalanchego-path"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testLocalNodeName = "e2eSubnetTest-local-node"
)

func createEtnaSubnetEvmConfig() {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"create",
		subnetName,
		"--evm",
		"--proof-of-authority",
		"--poa-manager-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func destroyLocalNode() string {
	_, err := os.Stat(testLocalNodeName)
	if os.IsNotExist(err) {
		return ""
	}
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"destroy",
		testLocalNodeName,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

func deployEtnaSubnet() string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet on etna devnet with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		subnetName,
		"--etna-devnet",
		"--use-local-machine",
		avalancheGoPath+"="+utils.EtnaAvalancheGoBinaryPath,
		"--num-local-nodes=1",
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

var _ = ginkgo.Describe("[Etna Subnet SOV]", func() {
	ginkgo.BeforeEach(func() {
		// key
		_ = utils.DeleteKey(keyName)
		output, err := commands.CreateKeyFromPath(keyName, utils.EwoqKeyPath)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		// subnet config
		_ = utils.DeleteConfigs(subnetName)
		_ = destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		_ = destroyLocalNode()
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})
	ginkgo.It("Create Etna Subnet Config & Deploy the Subnet To Public Etna On Local Machine", func() {
		createEtnaSubnetEvmConfig()
		deployEtnaSubnet()
	})
})

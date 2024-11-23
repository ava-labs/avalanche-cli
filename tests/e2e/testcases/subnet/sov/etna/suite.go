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

func createEtnaSubnetEvmConfig(poa, pos bool) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	cmdArgs := []string{
		"blockchain",
		"create",
		subnetName,
		"--evm",
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--proxy-contract-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--" + constants.SkipUpdateFlag,
	}
	if poa {
		cmdArgs = append(cmdArgs, "--proof-of-authority")
	} else if pos {
		cmdArgs = append(cmdArgs, "--proof-of-stake")
	}

	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func destroyLocalNode() {
	_, err := os.Stat(testLocalNodeName)
	if os.IsNotExist(err) {
		return
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
}

func deployEtnaSubnetEtnaFlag() {
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
		"--num-local-nodes=1",
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

func deployEtnaSubnetEtnaFlagConvertOnly() {
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
		"--num-local-nodes=1",
		"--convert-only",
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

func deployEtnaSubnetClusterFlagConvertOnly(clusterName string) {
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
		fmt.Sprintf("--cluster=%s", clusterName),
		"--convert-only",
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

func initValidatorManagerClusterFlag(
	subnetName string,
	clusterName string,
) error {
	cmd := exec.Command(
		CLIBinary,
		"contract",
		"initValidatorManager",
		subnetName,
		"--cluster",
		clusterName,
		"--genesis-key",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return err
}

func initValidatorManagerEtnaFlag(
	subnetName string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"contract",
		"initValidatorManager",
		subnetName,
		"--etna-devnet",
		"--genesis-key",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
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
		destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		destroyLocalNode()
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})
	ginkgo.It("Create Etna POA Subnet Config & Deploy the Subnet To Public Etna On Local Machine", func() {
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetEtnaFlag()
	})

	ginkgo.It("Create Etna POS Subnet Config & Deploy the Subnet To Public Etna On Local Machine", func() {
		createEtnaSubnetEvmConfig(false, true)
		deployEtnaSubnetEtnaFlag()
	})

	ginkgo.It("Start Local Node on Etna & Deploy the Subnet To Public Etna using cluster flag", func() {
		_, err := commands.CreateLocalEtnaDevnetNode(testLocalNodeName, 1)
		gomega.Expect(err).Should(gomega.BeNil())
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetClusterFlagConvertOnly(testLocalNodeName)
		_, err = commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = initValidatorManagerClusterFlag(subnetName, testLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("Mix and match network and cluster flags test 1", func() {
		_, err := commands.CreateLocalEtnaDevnetNode(testLocalNodeName, 1)
		gomega.Expect(err).Should(gomega.BeNil())
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetClusterFlagConvertOnly(testLocalNodeName)
		_, err = commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = initValidatorManagerEtnaFlag(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})
	ginkgo.It("Mix and match network and cluster flags test 2", func() {
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetEtnaFlagConvertOnly()
		_, err := commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = initValidatorManagerClusterFlag(subnetName, testLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
	})
})

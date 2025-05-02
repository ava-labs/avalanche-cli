// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
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
	keyName           = "ewoq"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
)

func createEtnaSubnetEvmConfig(poa, pos bool) string {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	cmdArgs := []string{
		"blockchain",
		"create",
		utils.BlockchainName,
		"--evm",
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--proxy-contract-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--icm=false",
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
	exists, err = utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// return binary versions for this conf
	mapper := utils.NewVersionMapper()
	mapping, err := utils.GetVersionMapping(mapper)
	gomega.Expect(err).Should(gomega.BeNil())
	return mapping[utils.LatestAvago2EVMKey]
}

func createEtnaSubnetEvmConfigWithoutProxyOwner(poa, pos bool) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	cmdArgs := []string{
		"blockchain",
		"create",
		utils.BlockchainName,
		"--evm",
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--icm=false",
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
	exists, err = utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func createEtnaSubnetEvmConfigValidatorManagerFlagKeyname(poa, pos bool) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	cmdArgs := []string{
		"blockchain",
		"create",
		utils.BlockchainName,
		"--evm",
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--proxy-contract-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--icm=false",
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
	exists, err = utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func createEtnaSubnetEvmConfigValidatorManagerFlagPChain(poa, pos bool) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	cmdArgs := []string{
		"blockchain",
		"create",
		utils.BlockchainName,
		"--evm",
		"--validator-manager-owner",
		ewoqPChainAddress,
		"--proxy-contract-owner",
		ewoqPChainAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--icm=false",
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
	gomega.Expect(err).ShouldNot(gomega.BeNil())
}

func destroyLocalNode() {
	_, err := os.Stat(utils.TestLocalNodeName)
	if os.IsNotExist(err) {
		return
	}
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"destroy",
		utils.TestLocalNodeName,
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
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet on etna devnet with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		utils.BlockchainName,
		"--local",
		"--num-bootstrap-validators=1",
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
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet on etna devnet with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		utils.BlockchainName,
		"--local",
		"--num-bootstrap-validators=1",
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
	exists, err := utils.SubnetConfigExists(utils.BlockchainName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet on etna devnet with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		utils.BlockchainName,
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
		"--local",
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

var avagoVersion string

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
		_ = utils.DeleteConfigs(utils.BlockchainName)
		destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		destroyLocalNode()
		commands.DeleteSubnetConfig(utils.BlockchainName)
		_ = utils.DeleteKey(keyName)
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("Test Create Etna POA Subnet Config With Key Name for Validator Manager Flag", func() {
		createEtnaSubnetEvmConfigValidatorManagerFlagKeyname(true, false)
	})

	ginkgo.It("Test Create Etna POA Subnet Config Without Proxy Owner Flag", func() {
		createEtnaSubnetEvmConfigWithoutProxyOwner(true, false)
	})

	ginkgo.It("Create Etna POA Subnet Config & Deploy the Subnet To Etna Local Network On Local Machine", func() {
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetEtnaFlag()
	})

	ginkgo.It("Create Etna POS Subnet Config & Deploy the Subnet To Etna Local Network On Local Machine", func() {
		createEtnaSubnetEvmConfig(false, true)
		deployEtnaSubnetEtnaFlag()
	})

	ginkgo.It("Start Local Node on Etna & Deploy the Subnet To Etna Local Network using cluster flag", func() {
		avagoVersion = createEtnaSubnetEvmConfig(true, false)
		_ = commands.StartNetworkWithParams(map[string]string{
			"version": avagoVersion,
		})
		_, err := commands.CreateLocalEtnaNode(avagoVersion, utils.TestLocalNodeName, 1)
		gomega.Expect(err).Should(gomega.BeNil())
		deployEtnaSubnetClusterFlagConvertOnly(utils.TestLocalNodeName)
		_, err = commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = initValidatorManagerClusterFlag(utils.BlockchainName, utils.TestLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("Mix and match network and cluster flags test 1", func() {
		avagoVersion = createEtnaSubnetEvmConfig(true, false)
		_ = commands.StartNetworkWithParams(map[string]string{
			"version": avagoVersion,
		})
		_, err := commands.CreateLocalEtnaNode(avagoVersion, utils.TestLocalNodeName, 1)
		gomega.Expect(err).Should(gomega.BeNil())
		deployEtnaSubnetClusterFlagConvertOnly(utils.TestLocalNodeName)
		_, err = commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = initValidatorManagerEtnaFlag(utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
	})
	ginkgo.It("Mix and match network and cluster flags test 2", func() {
		createEtnaSubnetEvmConfig(true, false)
		deployEtnaSubnetEtnaFlagConvertOnly()
		_, err := commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = initValidatorManagerClusterFlag(utils.BlockchainName, utils.TestLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
	})
})

var _ = ginkgo.Describe("[Etna Subnet SOV With Errors]", func() {
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
		_ = utils.DeleteConfigs(utils.BlockchainName)
		destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		_, _ = commands.CleanNetwork()
	})

	ginkgo.It("Test Create Etna POA Subnet Config With P Chain Address for Validator Manager Flag", func() {
		createEtnaSubnetEvmConfigValidatorManagerFlagPChain(true, false)
	})
})

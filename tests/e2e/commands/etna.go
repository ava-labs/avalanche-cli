// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

const (
	etnaDevnetFlag = "--etna-devnet"
)

func CreateEtnaSubnetEvmConfig(
	subnetName string,
	ewoqEVMAddress string,
) {
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

func CreateLocalEtnaDevnetNode(
	clusterName string,
	numNodes int,
	avalanchegoPath string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"start",
		clusterName,
		etnaDevnetFlag,
		"--num-nodes",
		fmt.Sprintf("%d", numNodes),
		"--avalanchego-path",
		avalanchegoPath,
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

func DestroyLocalNode(
	clusterName string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"destroy",
		clusterName,
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

func DeployEtnaSubnetToCluster(
	subnetName string,
	ewoqPChainAddress string,
	clusterName string,
	bootstrapEndpoints []string,
	convertOnly bool,
) (string, error) {
	convertOnlyFlag := ""
	if convertOnly {
		convertOnlyFlag = "--convert-only"
	}
	bootstrapEndpointsFlag := ""
	if len(bootstrapEndpoints) > 0 {
		bootstrapEndpointsFlag = "--bootstrap-endpoints=" + strings.Join(bootstrapEndpoints, ",")
	}
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
		"--cluster",
		clusterName,
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		bootstrapEndpointsFlag,
		convertOnlyFlag,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

func TrackLocalEtnaSubnet(
	clusterName string,
	subnetName string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"track",
		clusterName,
		subnetName,
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

func AddEtnaSubnetValidatorToCluster(
	clusterName string,
	subnetName string,
	nodeEndpoint string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"addValidator",
		subnetName,
		"--cluster",
		clusterName,
		"--node-endpoint",
		nodeEndpoint,
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

func RemoveEtnaSubnetValidatorFromCluster(
	clusterName string,
	subnetName string,
	nodeEndpoint string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"removeValidator",
		subnetName,
		"--cluster",
		clusterName,
		"--node-endpoint",
		nodeEndpoint,
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

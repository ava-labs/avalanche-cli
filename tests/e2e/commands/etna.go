// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

type SubnetManagementType uint

const (
	Unknown SubnetManagementType = iota
	PoA
	PoSNative
	PoSERC20
)

const (
	PoAString       = "proof-of-authority"
	PoSNativeString = "proof-of-stake-native"
	PoSERC20String  = "proof-of-stake-erc20"
	// TODO: when mapping is compatible, remove these constants
	GraniteFujiAvagoVersion     = "v1.14.0"
	GraniteFujiSubnetEVMVersion = "v0.8.0"
)

func CreateEtnaSubnetEvmConfig(
	subnetName string,
	ewoqEVMAddress string,
	subnetManagementType SubnetManagementType,
) (string, string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	subnetManagementStr := PoAString
	if subnetManagementType == PoSNative {
		subnetManagementStr = PoSNativeString
	} else if subnetManagementType == PoSERC20 {
		subnetManagementStr = PoSERC20String
	}
	// Create config
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"create",
		subnetName,
		"--evm",
		fmt.Sprintf("--%s", subnetManagementStr),
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--proxy-contract-owner",
		ewoqEVMAddress,
		"--test-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--icm=false",
		"--"+constants.SkipUpdateFlag,
		"--local",
	)
	fmt.Println(cmd)
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

	// return binary versions for this conf
	mapper := utils.NewVersionMapper()
	mapping, err := utils.GetVersionMapping(mapper)
	gomega.Expect(err).Should(gomega.BeNil())
	// TODO: when compatible, return mapping[utils.LatestEVM2AvagoKey], mapping[utils.LatestAvago2EVMKey]
	return mapping[utils.LatestEVM2AvagoKey], GraniteFujiAvagoVersion
}

func CreateLocalEtnaNode(
	avagoVersion string,
	clusterName string,
	numNodes int,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"start",
		clusterName,
		"--local",
		"--num-nodes",
		fmt.Sprintf("%d", numNodes),
		"--custom-avalanchego-version",
		avagoVersion,
		"--"+constants.SkipUpdateFlag,
	)
	fmt.Println(cmd)
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

func DeployEtnaBlockchain(
	subnetName string,
	clusterName string,
	bootstrapEndpoints []string,
	ewoqPChainAddress string,
	convertOnly bool,
	externalManager bool,
	erc20TokenAddress string,
	rewardBasisPoints uint64,
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
	args := []string{
		"blockchain",
		"deploy",
		subnetName,
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--" + constants.SkipUpdateFlag,
	}
	if clusterName != "" {
		args = append(args, "--cluster", clusterName)
	} else {
		args = append(args, "--local")
	}
	if convertOnlyFlag != "" {
		args = append(args, convertOnlyFlag)
	}
	if bootstrapEndpointsFlag != "" {
		args = append(args, bootstrapEndpointsFlag)
	}
	if externalManager {
		args = append(args, "--vmc-c-chain", "--vmc-genesis-key")
	}
	if erc20TokenAddress != "" {
		args = append(args, "--erc20-token-address", erc20TokenAddress)
		// If we're providing an ERC20 token address, we need to transfer ownership
		// Since we deployed with genesis key (ewoq), use ewoq key for ownership transfer
		args = append(args, "--erc20-token-owner-key", "ewoq")
	}
	if rewardBasisPoints > 0 {
		args = append(args, "--reward-basis-points", fmt.Sprintf("%d", rewardBasisPoints))
	}
	cmd := exec.Command(CLIBinary, args...)
	fmt.Println(cmd)
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
		"node",
		"local",
		"track",
		clusterName,
		subnetName,
		"--"+constants.SkipUpdateFlag,
	)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

func DeployValidatorManagerToCChain(
	subnetManagementType SubnetManagementType,
	rewardBasisPoints uint64,
) (string, error) {
	args := []string{
		"contract",
		"deploy",
		"validatorManager",
		"--local",
		"--c-chain",
		"--genesis-key",
		"--deploy-proxy",
		"--proxy-owner-genesis-key",
		"--" + constants.SkipUpdateFlag,
	}

	// Add subnet management type flag
	switch subnetManagementType {
	case PoA:
		args = append(args, "--poa")
	case PoSNative:
		args = append(args, "--pos-native")
	case PoSERC20:
		args = append(args, "--pos-erc20")
	}

	if rewardBasisPoints > 0 {
		args = append(
			args,
			"--reward-basis-points",
			fmt.Sprintf("%d", rewardBasisPoints),
		)
	}

	cmd := exec.Command(CLIBinary, args...)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

func DeployERC20TokenAtCChain(
	tokenSymbol string,
	tokenSupply uint64,
	fundedAddress string,
) (string, error) {
	args := []string{
		"contract",
		"deploy",
		"erc20",
		"--local",
		"--c-chain",
		"--genesis-key",
		"--symbol",
		tokenSymbol,
		"--supply",
		fmt.Sprintf("%d", tokenSupply),
		"--funded",
		fundedAddress,
		"--mintable",
		"--" + constants.SkipUpdateFlag,
	}

	cmd := exec.Command(CLIBinary, args...)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

func InitValidatorManager(
	subnetName string,
	clusterName string,
	endpoint string,
	blockchainID string,
	rewardBasisPoints uint64,
	erc20TokenAddress string,
	rewardCalculatorAddress string,
) (string, error) {
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
	if endpoint != "" {
		cmd.Args = append(
			cmd.Args,
			"--endpoint",
			endpoint,
			"--rpc",
			fmt.Sprintf("%s/ext/bc/%s/rpc", endpoint, blockchainID),
		)
	}
	if rewardBasisPoints > 0 {
		cmd.Args = append(
			cmd.Args,
			"--pos-reward-basis-points",
			fmt.Sprintf("%d", rewardBasisPoints),
		)
	}
	if erc20TokenAddress != "" {
		cmd.Args = append(
			cmd.Args,
			"--erc20-token-address",
			erc20TokenAddress,
			"--erc20-token-owner-key",
			"ewoq",
		)
	}
	if rewardCalculatorAddress != "" {
		cmd.Args = append(
			cmd.Args,
			"--pos-reward-calculator-address",
			rewardCalculatorAddress,
		)
	}
	fmt.Println(cmd)
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
	ewoqPChainAddress string,
	balance int,
	createLocalValidator bool,
	weight int,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"addValidator",
		subnetName,
		"--ewoq",
		"--balance",
		strconv.Itoa(balance),
		"--remaining-balance-owner",
		ewoqPChainAddress,
		"--disable-owner",
		ewoqPChainAddress,
		"--delegation-fee",
		"100",
		"--rewards-recipient",
		utils.EwoqEVMAddress,
		"--staking-period",
		"100s",
		"--weight",
		strconv.Itoa(weight),
		"--staker-genesis-key",
		"--"+constants.SkipUpdateFlag,
	)
	if clusterName != "" {
		cmd.Args = append(cmd.Args, "--cluster", clusterName)
	} else {
		cmd.Args = append(cmd.Args, "--local")
	}
	if nodeEndpoint != "" {
		cmd.Args = append(cmd.Args, "--node-endpoint", nodeEndpoint)
	}
	if createLocalValidator {
		cmd.Args = append(cmd.Args, "--create-local-validator")
	}
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func RemoveEtnaSubnetValidatorFromCluster(
	clusterName string,
	subnetName string,
	nodeEndpoint string,
	keyName string,
	uptimeSec uint64,
	force bool,
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
		"--key",
		keyName,
		"--uptime",
		strconv.Itoa(int(uptimeSec)),
		"--staker-genesis-key",
		"--"+constants.SkipUpdateFlag,
	)
	if force {
		cmd.Args = append(cmd.Args, "--force")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

func GetLocalClusterStatus(
	clusterName string,
	blockchainName string,
) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"status",
		clusterName,
		"--"+constants.SkipUpdateFlag,
	)
	if blockchainName != "" {
		cmd.Args = append(cmd.Args, "--blockchain", blockchainName)
	}
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output), err
}

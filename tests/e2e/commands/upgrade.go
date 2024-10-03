// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

func ImportUpgradeBytes(subnetName, filepath string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"import",
		subnetName,
		"--upgrade-filepath",
		filepath,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func UpgradeVMConfig(subnetName string, targetVersion string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--config",
		"--version",
		targetVersion,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func UpgradeCustomVM(subnetName string, binaryPath string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--config",
		"--binary",
		binaryPath,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.Output()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func UpgradeVMPublic(subnetName string, targetVersion string, pluginDir string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--fuji",
		"--version",
		targetVersion,
		"--plugin-dir",
		pluginDir,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func UpgradeVMLocal(subnetName string, targetVersion string) string {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--local",
		"--version",
		targetVersion,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}

	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func UpgradeCustomVMLocal(subnetName string, binaryPath string) string {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--local",
		"--binary",
		binaryPath,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.Output()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func ApplyUpgradeLocal(subnetName string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"apply",
		subnetName,
		"--local",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func ApplyUpgradeToPublicNode(subnetName, avagoChainConfDir string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"apply",
		subnetName,
		"--fuji",
		"--avalanchego-chain-config-dir",
		avagoChainConfDir,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

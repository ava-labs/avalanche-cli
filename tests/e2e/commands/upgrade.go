// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
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
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

/* #nosec G204 */
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
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

/* #nosec G204 */
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
	)

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
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

func ApplyUpgradeLocal(subnetName string) (string, error) {
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"apply",
		subnetName,
		"--local",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"
)

/* #nosec G204 */
func UpgradeVMFuture(subnetName string, targetVersion string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		UpgradeCmd,
		"vm",
		subnetName,
		"--future",
		"--version",
		targetVersion,
	)

	fmt.Println(cmd.String())

	output, err := cmd.Output()
	exitErr, typeOk := err.(*exec.ExitError)
	stderr := ""
	if typeOk {
		stderr = string(exitErr.Stderr)
	}
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
		fmt.Println(stderr)
	}
	return string(output), err
}

func UpgradeVMPublic(subnetName string, targetVersion string, pluginDir string) (string, error) {
	// Create config
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

	fmt.Println(cmd.String())

	output, err := cmd.Output()
	exitErr, typeOk := err.(*exec.ExitError)
	stderr := ""
	if typeOk {
		stderr = string(exitErr.Stderr)
	}
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		fmt.Println(err)
		fmt.Println(stderr)
	}
	return string(output), err
}

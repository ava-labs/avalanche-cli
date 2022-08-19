// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CreateSubnetConfig(subnetName string, genesisPath string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"create",
		"--genesis",
		genesisPath,
		"--evm",
		subnetName,
		"--latest",
	)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func CreateSubnetConfigWithVersion(subnetName string, genesisPath string, version string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"create",
		"--genesis",
		genesisPath,
		"--evm",
		subnetName,
		"--vm-version",
		version,
	)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func CreateCustomVMSubnetConfig(subnetName string, genesisPath string, vmPath string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())
	// Check vm binary does not already exist
	exists, err = utils.SubnetCustomVMExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"create",
		"--genesis",
		genesisPath,
		"--vm",
		vmPath,
		"--custom",
		subnetName,
	)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
	exists, err = utils.SubnetCustomVMExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func DeleteSubnetConfig(subnetName string) {
	// Config should exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Now delete config
	cmd := exec.Command(CLIBinary, SubnetCmd, "delete", subnetName)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should no longer exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocally(subnetName string) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet locally
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"deploy",
		"--local",
		subnetName,
	)
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
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocallyWithVersion(subnetName string, version string) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet locally
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"deploy",
		"--local",
		subnetName,
		"--avalanchego-version",
		version,
	)
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
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

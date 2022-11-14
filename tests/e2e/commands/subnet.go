// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CreateSubnetEvmConfig(subnetName string, genesisPath string) {
	CreateSubnetEvmConfigWithVersion(subnetName, genesisPath, utils.SubnetEVMVersion)
}

/* #nosec G204 */
func CreateSubnetEvmConfigWithVersion(subnetName string, genesisPath string, version string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmdArgs := []string{SubnetCmd, "create", "--genesis", genesisPath, "--evm", subnetName}
	if version == "" {
		cmdArgs = append(cmdArgs, "--latest")
	} else {
		cmdArgs = append(cmdArgs, "--vm-version", version)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func CreateSpacesVMConfig(subnetName string, genesisPath string) {
	CreateSpacesVMConfigWithVersion(subnetName, genesisPath, utils.SpacesVMVersion)
}

/* #nosec G204 */
func CreateSpacesVMConfigWithVersion(subnetName string, genesisPath string, version string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmdArgs := []string{SubnetCmd, "create", "--genesis", genesisPath, "--spacesvm", subnetName}
	if version == "" {
		cmdArgs = append(cmdArgs, "--latest")
	} else {
		cmdArgs = append(cmdArgs, "--vm-version", version)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func CreateCustomVMConfig(subnetName string, genesisPath string, vmPath string) {
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
	output, err := cmd.CombinedOutput()
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should no longer exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocally(subnetName string) string {
	return DeploySubnetLocallyWithArgs(subnetName, "", "")
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocallyExpectError(subnetName string) {
	DeploySubnetLocallyWithArgsExpectError(subnetName, utils.AvagoVersion, "")
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocallyWithViperConf(subnetName string, confPath string) string {
	return DeploySubnetLocallyWithArgs(subnetName, utils.AvagoVersion, confPath)
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocallyWithVersion(subnetName string, version string) string {
	return DeploySubnetLocallyWithArgs(subnetName, version, "")
}

// Returns the deploy output
/* #nosec G204 */
func DeploySubnetLocallyWithArgs(subnetName string, version string, confPath string) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet locally
	cmdArgs := []string{SubnetCmd, "deploy", "--local", subnetName}
	if version != "" {
		cmdArgs = append(cmdArgs, "--avalanchego-version", version)
	}
	if confPath != "" {
		cmdArgs = append(cmdArgs, "--config", confPath)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
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
func DeploySubnetLocallyWithArgsExpectError(subnetName string, version string, confPath string) {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet locally
	cmdArgs := []string{SubnetCmd, "deploy", "--local", subnetName}
	if version != "" {
		cmdArgs = append(cmdArgs, "--avalanchego-version", version)
	}
	if confPath != "" {
		cmdArgs = append(cmdArgs, "--config", confPath)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	_, err = cmd.CombinedOutput()
	gomega.Expect(err).Should(gomega.HaveOccurred())
}

// simulates fuji deploy execution path on a local network
func SimulateFujiDeploy(
	subnetName string,
	key string,
	controlKeys string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	// Deploy subnet locally
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"deploy",
		"--fuji",
		"--threshold",
		"1",
		"--key",
		key,
		"--control-keys",
		controlKeys,
		subnetName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

// simulates mainnet deploy execution path on a local network
func SimulateMainnetDeploy(
	subnetName string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	// Deploy subnet locally
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"deploy",
		"--mainnet",
		"--threshold",
		"1",
		"--same-control-key",
		subnetName,
	)
	stdoutPipe, err := cmd.StdoutPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	stderrPipe, err := cmd.StderrPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	err = cmd.Start()
	gomega.Expect(err).Should(gomega.BeNil())

	stdout := ""
	go func(p io.ReadCloser) {
		reader := bufio.NewReader(p)
		line, err := reader.ReadString('\n')
		for err == nil {
			stdout += line
			fmt.Print(line)
			line, err = reader.ReadString('\n')
		}
	}(stdoutPipe)

	stderr, err := io.ReadAll(stderrPipe)
	gomega.Expect(err).Should(gomega.BeNil())
	fmt.Println(string(stderr))

	err = cmd.Wait()
	gomega.Expect(err).Should(gomega.BeNil())

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	return stdout + string(stderr)
}

// simulates fuji add validator execution path on a local network
func SimulateFujiAddValidator(
	subnetName string,
	key string,
	nodeID string,
	start string,
	period string,
	weight string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"addValidator",
		"--fuji",
		"--key",
		key,
		"--nodeID",
		nodeID,
		"--start-time",
		start,
		"--staking-period",
		period,
		"--weight",
		weight,
		subnetName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

// simulates mainnet add validator execution path on a local network
func SimulateMainnetAddValidator(
	subnetName string,
	nodeID string,
	start string,
	period string,
	weight string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"addValidator",
		"--mainnet",
		"--nodeID",
		nodeID,
		"--start-time",
		start,
		"--staking-period",
		period,
		"--weight",
		weight,
		subnetName,
	)
	stdoutPipe, err := cmd.StdoutPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	stderrPipe, err := cmd.StderrPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	err = cmd.Start()
	gomega.Expect(err).Should(gomega.BeNil())

	stdout := ""
	go func(p io.ReadCloser) {
		reader := bufio.NewReader(p)
		line, err := reader.ReadString('\n')
		for err == nil {
			stdout += line
			fmt.Print(line)
			line, err = reader.ReadString('\n')
		}
	}(stdoutPipe)

	stderr, err := io.ReadAll(stderrPipe)
	gomega.Expect(err).Should(gomega.BeNil())
	fmt.Println(string(stderr))

	err = cmd.Wait()
	gomega.Expect(err).Should(gomega.BeNil())

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	return stdout + string(stderr)
}

// simulates fuji join execution path on a local network
func SimulateFujiJoin(
	subnetName string,
	avalanchegoConfig string,
	pluginDir string,
	nodeID string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"join",
		"--fuji",
		"--avalanchego-config",
		avalanchegoConfig,
		"--plugin-dir",
		pluginDir,
		"--force-whitelist-check",
		"--fail-if-not-validating",
		"--nodeID",
		nodeID,
		"--force-write",
		subnetName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

// simulates mainnet join execution path on a local network
func SimulateMainnetJoin(
	subnetName string,
	avalanchegoConfig string,
	pluginDir string,
	nodeID string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// enable simulation of public network execution paths on a local network
	os.Setenv(constants.SimulatePublicNetwork, "true")

	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"join",
		"--mainnet",
		"--avalanchego-config",
		avalanchegoConfig,
		"--plugin-dir",
		pluginDir,
		"--force-whitelist-check",
		"--fail-if-not-validating",
		"--nodeID",
		nodeID,
		"--force-write",
		subnetName,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}

	// disable simulation of public network execution paths on a local network
	os.Unsetenv(constants.SimulatePublicNetwork)

	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

/* #nosec G204 */
func ImportSubnetConfig(repoAlias string, subnetName string) {
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
		"import",
		"--repo",
		repoAlias,
		"--subnet",
		subnetName,
	)
	output, err := cmd.CombinedOutput()
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

	// Config should now exist
	exists, err = utils.APMConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
	exists, err = utils.SubnetAPMVMExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

/* #nosec G204 */
func ImportSubnetConfigFromURL(repoURL string, branch string, subnetName string) {
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
		"import",
		"--repo",
		repoURL,
		"--branch",
		branch,
		"--subnet",
		subnetName,
	)
	output, err := cmd.CombinedOutput()
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

	// Config should now exist
	exists, err = utils.APMConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
	exists, err = utils.SubnetAPMVMExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func SimulateGetSubnetStatsFuji(subnetName, subnetID string) string {
	// Check config does already exist:
	// We want to run stats on an existing subnet
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// add the subnet ID to the `fuji` section so that the `stats` command
	// can find it (as this is a simulation with a `local` network,
	// it got written in to the `local` network section)
	err = utils.AddSubnetIDToSidecar(subnetName, models.Fuji, subnetID)
	gomega.Expect(err).Should(gomega.BeNil())
	// run stats
	cmd := exec.Command(
		CLIBinary,
		SubnetCmd,
		"stats",
		subnetName,
		"--fuji",
	)
	output, err := cmd.CombinedOutput()
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
	gomega.Expect(exitErr).Should(gomega.BeNil())
	return string(output)
}

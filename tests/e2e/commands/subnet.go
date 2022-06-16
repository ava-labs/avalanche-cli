package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

func CreateSubnetConfig(subnetName string, genesisPath string) {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmd := exec.Command(
		utils.CLIBinary,
		SubnetCmd,
		"create",
		"--file",
		genesisPath,
		"--evm",
		subnetName,
	)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func DeleteSubnetConfig(subnetName string) {
	// Config should exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Now delete config
	cmd := exec.Command(utils.CLIBinary, SubnetCmd, "delete", subnetName)
	_, err = cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should no longer exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())
}

// Returns the deploy output
func DeploySubnetLocally(subnetName string) string {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Create config
	cmd := exec.Command(
		utils.CLIBinary,
		SubnetCmd,
		"deploy",
		"--local",
		subnetName,
	)
	fmt.Println(cmd.String())
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

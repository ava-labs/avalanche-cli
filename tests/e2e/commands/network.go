package commands

import (
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

// Returns the deploy output
func CleanNetwork() {
	// Create config
	cmd := exec.Command(
		utils.CLIBinary,
		NetworkCmd,
		"clean",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

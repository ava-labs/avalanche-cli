package commands

import (
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

func CleanNetwork() {
	cmd := exec.Command(
		utils.CLIBinary,
		NetworkCmd,
		"clean",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

func StartNetwork() string {
	cmd := exec.Command(
		utils.CLIBinary,
		NetworkCmd,
		"start",
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func StopNetwork() {
	cmd := exec.Command(
		utils.CLIBinary,
		NetworkCmd,
		"stop",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

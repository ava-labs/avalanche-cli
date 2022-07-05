package commands

import (
	"os/exec"

	"github.com/onsi/gomega"
)

func GetVersion() string {
	/* #nosec G204 */
	cmd := exec.Command(CLIBinary, "--version")
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

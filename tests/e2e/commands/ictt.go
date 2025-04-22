package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

const (
	ICTTCmd = "ictt"
)

/* #nosec G204 */
func DeployICTT(network, subnet string) string {
	// Create config
	cmdArgs := []string{
		ICTTCmd,
		"deploy",
		network,
		"--c-chain-home",
		"--remote-blockchain",
		subnet,
		"--deploy-native-home",
		"--home-genesis-key",
		"--remote-genesis-key",
		"--" + constants.SkipUpdateFlag,
	}

	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

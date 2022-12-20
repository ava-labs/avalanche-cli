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
func CleanNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

/* #nosec G204 */
func CleanNetworkHard() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
		"--hard",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

/* #nosec G204 */
func StartNetwork() string {
	mapper := utils.NewVersionMapper()
	mapping, err := utils.GetVersionMapping(mapper)
	gomega.Expect(err).Should(gomega.BeNil())

	return StartNetworkWithVersion(mapping[utils.OnlyAvagoKey])
}

/* #nosec G204 */
func StartNetworkWithVersion(version string) string {
	cmdArgs := []string{NetworkCmd, "start"}
	if version != "" {
		cmdArgs = append(cmdArgs, "--avalanchego-version", version)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

/* #nosec G204 */
func StopNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"stop",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

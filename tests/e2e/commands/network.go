// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

func CleanNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

func CleanNetworkHard() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
		"--hard",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

func StartNetwork() string {
	mapper := utils.NewVersionMapper()
	mapping, err := utils.GetVersionMapping(mapper)
	gomega.Expect(err).Should(gomega.BeNil())

	return StartNetworkWithVersion(mapping[utils.OnlyAvagoKey])
}

func StartNetworkWithVersion(version string) string {
	cmdArgs := []string{NetworkCmd, "start"}
	cmdArgs = append(cmdArgs, "--"+constants.SkipUpdateFlag)
	if version != "" {
		cmdArgs = append(
			cmdArgs,
			"--avalanchego-version",
			version,
		)
	}
	// in case we want to use specific avago for local tests
	debugAvalanchegoPath := os.Getenv(constants.E2EDebugAvalanchegoPath)
	if debugAvalanchegoPath != "" {
		cmdArgs = append(cmdArgs, "--avalanchego-path", debugAvalanchegoPath)
	}
	cmd := exec.Command(CLIBinary, cmdArgs...) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func StopNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"stop",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
}

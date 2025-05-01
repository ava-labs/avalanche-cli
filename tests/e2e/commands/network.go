// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CleanNetwork() (string, error) {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

/* #nosec G204 */
func StartNetwork() string {
	return StartNetworkWithVersion("")
}

func StartNetworkWithNodeNumber(numOfNodes uint) string {
	return startNetworkWithParams(map[string]string{
		"number-of-nodes": strconv.FormatUint(uint64(numOfNodes), 10),
	})
}

/* #nosec G204 */
func StartNetworkWithVersion(version string) string {
	return startNetworkWithParams(map[string]string{
		"version": version,
	})
}

func startNetworkWithParams(paramMap map[string]string) string {
	cmdArgs := []string{NetworkCmd, "start"}
	cmdArgs = append(cmdArgs, "--"+constants.SkipUpdateFlag)

	for k, v := range paramMap {
		switch k {
		case "version":
			if v != "" {
				cmdArgs = append(
					cmdArgs,
					"--avalanchego-version",
					v,
				)
			}
		case "number-of-nodes":
			cmdArgs = append(
				cmdArgs,
				"--num-nodes",
				v,
			)
		}
	}

	// in case we want to use specific avago for local tests
	debugAvalanchegoPath := os.Getenv(constants.E2EDebugAvalancheGoPath)
	if debugAvalanchegoPath != "" {
		cmdArgs = append(cmdArgs, "--avalanchego-path", debugAvalanchegoPath)
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
func StopNetwork(stopCmdFlags ...string) error {
	stopCmdFlasg := append([]string{
		NetworkCmd,
		"stop",
		"--" + constants.SkipUpdateFlag,
	}, stopCmdFlags...)
	cmd := exec.Command(
		CLIBinary,
		stopCmdFlasg...,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return err
}

func GetNetworkStatus() (string, error) {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"status",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	return string(output), err
}

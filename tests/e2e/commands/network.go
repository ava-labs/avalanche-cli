// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CleanNetwork() (string, error) {
	output, err := utils.TestCommand(
		NetworkCmd,
		"clean",
		[]string{
			"--" + constants.SkipUpdateFlag,
		},
		utils.GlobalFlags{},
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	return output, err
}

func CleanNetworkHard() (string, error) {
	output, err := utils.TestCommand(
		NetworkCmd,
		"clean",
		[]string{
			"--hard",
			"--" + constants.SkipUpdateFlag,
		},
		utils.GlobalFlags{},
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	return output, err
}

/* #nosec G204 */
func StartNetwork() string {
	return StartNetworkWithParams(map[string]string{
		"version": "",
	})
}

func StartNetworkWithParams(paramMap map[string]string) string {
	cmdArgs := utils.GlobalFlags{}

	for k, v := range paramMap {
		switch k {
		case "version":
			if v != "" {
				cmdArgs["avalanchego-version"] = v
			}
		case "number-of-nodes":
			cmdArgs["num-nodes"] = v
		}
	}

	// in case we want to use specific avago for local tests
	debugAvalanchegoPath := os.Getenv(constants.E2EDebugAvalancheGoPath)
	if debugAvalanchegoPath != "" {
		cmdArgs["avalanchego-path"] = debugAvalanchegoPath
	}
	output, err := utils.TestCommand(
		NetworkCmd,
		"start",
		[]string{
			"--" + constants.SkipUpdateFlag,
		},
		cmdArgs,
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())
	return output
}

/* #nosec G204 */
func StopNetwork(stopCmdFlags ...string) error {
	output, err := utils.TestCommand(
		NetworkCmd,
		"stop",
		append([]string{
			"--" + constants.SkipUpdateFlag,
		}, stopCmdFlags...),
		utils.GlobalFlags{},
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	return err
}

func GetNetworkStatus() (string, error) {
	output, err := utils.TestCommand(
		NetworkCmd,
		"status",
		[]string{
			"--" + constants.SkipUpdateFlag,
		},
		utils.GlobalFlags{},
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	return output, err
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CleanNetwork() (string, error) {
	output, err := utils.TestCommand(
		utils.NetworkCmd,
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
		utils.NetworkCmd,
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
		utils.NetworkCmd,
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
		utils.NetworkCmd,
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

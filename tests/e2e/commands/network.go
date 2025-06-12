// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
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

/* #nosec G204 */
func StartNetwork() (string, error) {
	return StartNetworkWithParams(map[string]interface{}{})
}

func StartNetworkWithParams(paramMap map[string]interface{}) (string, error) {
	// in case we want to use specific avago for local tests
	debugAvalanchegoPath := os.Getenv(constants.E2EDebugAvalancheGoPath)
	if debugAvalanchegoPath != "" {
		paramMap["avalanchego-path"] = debugAvalanchegoPath
	}
	output, err := utils.TestCommand(
		NetworkCmd,
		"start",
		[]string{
			"--" + constants.SkipUpdateFlag,
		},
		paramMap,
		utils.TestFlags{},
	)
	if err != nil {
		fmt.Println(output)
		utils.PrintStdErr(err)
	}
	return output, err
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

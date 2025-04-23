// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commandse2e

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
)

type TestFlags map[string]string

// CommandGroup represents the different command groups available in the CLI
type CommandGroup string

const (
	BlockchainCmd CommandGroup = "blockchain"
)

var avalancheBinaryPath = "./bin/avalanche"

var GlobalFlags = map[string]interface{}{
	"local":             true,
	"skip-icm-deploy":   true,
	"skip-update-check": true,
}

// TestCommand tests a CLI command with flag inputs
func TestCommand(commandGroup CommandGroup, command string, args []string, testFlags TestFlags) (string, error) {
	// Build command arguments
	cmdArgs := []string{string(commandGroup), command}

	// Append any additional arguments
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	// Create a map to store all flags, starting with global flags
	allFlags := make(map[string]interface{})
	for flag, value := range GlobalFlags {
		allFlags[flag] = value
	}

	// Override with test case specific flags if provided
	for flag, value := range testFlags {
		allFlags[flag] = value
	}

	// Add all flags to command arguments
	for flag, value := range allFlags {
		cmdArgs = append(cmdArgs, "--"+flag+"="+fmt.Sprintf("%v", value))
	}

	// Execute the command
	cmd := exec.Command(avalancheBinaryPath, cmdArgs...)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var (
			exitErr *exec.ExitError
			stderr  string
		)
		if errors.As(err, &exitErr) {
			stderr = string(exitErr.Stderr)
		}
		fmt.Println(string(output))
		utils.PrintStdErr(err)
		fmt.Println(stderr)
	}

	return string(output), err
}

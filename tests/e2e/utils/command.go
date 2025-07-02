// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/cmd"
	"golang.org/x/exp/maps"
)

type GlobalFlags map[string]interface{}

type TestFlags map[string]interface{}

var avalancheBinaryPath = "./bin/avalanche"

// TestCommand tests a CLI command with flag inputs
func TestCommand(commandGroup cmd.CommandGroup, command string, args []string, globalFlags GlobalFlags, testFlags TestFlags) (string, error) {
	// Build command arguments
	cmdArgs := []string{string(commandGroup), command}

	// Append any additional arguments
	cmdArgs = append(cmdArgs, args...)

	allFlags := make(map[string]interface{})
	maps.Copy(allFlags, globalFlags)

	// Override with test case specific flags if provided
	maps.Copy(allFlags, testFlags)

	// Add all flags to command arguments
	for flag, value := range allFlags {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%v", flag, value))
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
		PrintStdErr(err)
		if stderr != "" {
			fmt.Println(stderr)
		}
	}

	return string(output), err
}

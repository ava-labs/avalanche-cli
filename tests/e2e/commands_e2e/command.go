// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commands_e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
)

// TestCase represents a single test case configuration
type TestCase struct {
	Name          string            `json:"name"`
	Flags         map[string]string `json:"flags"`
	ExpectedError string            `json:"expectedError,omitempty"`
}

// TestJSONConfig represents the json configuration that contains cli command flag inputs
type TestJSONConfig struct {
	GlobalFlags  map[string]interface{} `json:"globalFlags"`
	HappyPath    []TestCase             `json:"happyPath"`
	NotHappyPath []TestCase             `json:"notHappyPath"`
}

// CommandGroup represents the different command groups available in the CLI
type CommandGroup string

const (
	BlockchainCmd CommandGroup = "blockchain"
)

var avalancheBinaryPath = "./bin/avalanche"

// TestCommandWithJSONConfig tests a CLI command with flag inputs from a JSON file
func TestCommandWithJSONConfig(commandGroup CommandGroup, command string, args []string, configPath string, testCase *TestCase) (string, error) {
	// Build command arguments
	cmdArgs := []string{string(commandGroup), command}

	// Read and parse the JSON config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestJSONConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	// Append any additional arguments
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	// Create a map to store all flags, starting with global flags
	allFlags := make(map[string]interface{})
	for flag, value := range config.GlobalFlags {
		allFlags[flag] = value
	}

	// Override with test case specific flags if provided
	if testCase != nil {
		for flag, value := range testCase.Flags {
			allFlags[flag] = value
		}
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
		return "", err
	}

	return string(output), nil
}

// ReadTestConfig reads and parses the test configuration from a JSON file
func ReadTestConfig(configPath string) (*TestJSONConfig, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestJSONConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

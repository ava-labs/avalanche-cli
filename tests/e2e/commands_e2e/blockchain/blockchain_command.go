package blockchain

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

var avalancheBinaryPath = "./bin/avalanche"

// SetAvalancheBinaryPath sets the path to the avalanche binary
func SetAvalancheBinaryPath(path string) {
	avalancheBinaryPath = path
}

// TestCommandWithJSONConfig tests a CLI command with flag inputs from a JSON file
func TestCommandWithJSONConfig(command string, configPath string, testCase *TestCase) (string, error) {
	blockchainCmd := "blockchain"

	// Read and parse the JSON config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestJSONConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	// Build command arguments
	cmdArgs := []string{blockchainCmd, command}

	// Add blockchain name from global flags
	if blockchainName, ok := config.GlobalFlags["blockchainName"].(string); ok {
		cmdArgs = append(cmdArgs, blockchainName)
	}

	// Create a map to store all flags, starting with global flags
	allFlags := make(map[string]interface{})
	for flag, value := range config.GlobalFlags {
		if flag != "blockchainName" {
			allFlags[flag] = value
		}
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

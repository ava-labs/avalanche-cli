// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	promptsMocks "github.com/ava-labs/avalanche-cli/pkg/prompts/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	gitCommand    = "git"
	versionFlag   = "--version"
	buildScript   = "./build.sh"
	initCommand   = "init"
	remoteCommand = "remote"
)

func TestCheckGitIsInstalled(t *testing.T) {
	tests := []struct {
		name           string
		gitAvailable   bool
		expectedError  bool
		expectedOutput []string
	}{
		{
			name:          "git is installed and available",
			gitAvailable:  true,
			expectedError: false,
		},
		{
			name:          "git is not installed",
			gitAvailable:  false,
			expectedError: true,
			expectedOutput: []string{
				"Git tool is not available. It is a necessary dependency for CLI to import a custom VM.",
				"Please follow install instructions at https://git-scm.com/book/en/v2/Getting-Started-Installing-Git and try again",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			var buf bytes.Buffer
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, &buf)

			// Store original execCommand to restore later
			originalExecCommand := execCommand
			defer func() {
				execCommand = originalExecCommand
			}()

			// Mock execCommand based on test scenario
			if tt.gitAvailable {
				// Mock successful git command
				execCommand = func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						return exec.Command("true") // Always succeeds
					}
					return originalExecCommand(name, args...)
				}
			} else {
				// Mock failing git command
				execCommand = func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						return exec.Command("false") // Always fails
					}
					return originalExecCommand(name, args...)
				}
			}

			// Call the function under test
			err := CheckGitIsInstalled()

			output := buf.String()

			// Assertions
			if tt.expectedError {
				require.Error(t, err, "Expected an error when git is not available")

				// Verify that the expected messages were printed
				for _, expectedMsg := range tt.expectedOutput {
					require.Contains(t, output, expectedMsg, "Expected message should appear in output")
				}
			} else {
				require.NoError(t, err, "Expected no error when git is available")

				// Verify no error messages were printed
				require.NotContains(t, output, "Git tool is not available", "Should not print error message when git is available")
			}
		})
	}
}

func TestCheckGitIsInstalledCommandStructure(t *testing.T) {
	// This test verifies that the correct command and arguments are used
	t.Run("correct git command is executed", func(t *testing.T) {
		// Initialize ux.Logger for testing
		ux.Logger = nil
		ux.NewUserLog(logging.NoLog{}, io.Discard)

		var capturedName string
		var capturedArgs []string

		// Store original execCommand to restore later
		originalExecCommand := execCommand
		defer func() {
			execCommand = originalExecCommand
		}()

		// Mock execCommand to capture the command details
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedName = name
			capturedArgs = args
			// Return a successful command
			return exec.Command("true")
		}

		// Call the function
		err := CheckGitIsInstalled()

		// Verify the correct command was called
		require.NoError(t, err)
		require.Equal(t, gitCommand, capturedName, "Should execute 'git' command")
		require.Equal(t, []string{versionFlag}, capturedArgs, "Should use '--version' argument")
	})
}

func TestCheckGitIsInstalledErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "git command succeeds",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "git command fails",
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			var buf bytes.Buffer
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, &buf)

			// Store original execCommand to restore later
			originalExecCommand := execCommand
			defer func() {
				execCommand = originalExecCommand
			}()

			// Mock execCommand based on test scenario
			if tt.shouldFail {
				execCommand = func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						return exec.Command("false") // Always fails
					}
					return originalExecCommand(name, args...)
				}
			} else {
				execCommand = func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						return exec.Command("true") // Always succeeds
					}
					return originalExecCommand(name, args...)
				}
			}

			// Call function
			err := CheckGitIsInstalled()

			// Assertions
			if tt.expectError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				require.Contains(t, buf.String(), "Git tool is not available", "Should print error message")
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tt.name)
				require.NotContains(t, buf.String(), "Git tool is not available", "Should not print error message")
			}
		})
	}
}

func TestCheckGitIsInstalledExecCommandFailures(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func() func(string, ...string) *exec.Cmd
		expectError bool
		description string
	}{
		{
			name: "command not found",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						// Simulate command not found
						return exec.Command("nonexistentcommandthatdoesnotexist12345")
					}
					return exec.Command(name, args...)
				}
			},
			expectError: true,
			description: "Should handle when git command is not found in PATH",
		},
		{
			name: "permission denied",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						// Simulate permission denied (try to execute a directory)
						return exec.Command("/usr")
					}
					return exec.Command(name, args...)
				}
			},
			expectError: true,
			description: "Should handle permission denied errors",
		},
		{
			name: "command exits with error code",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						// Simulate command that exits with error code
						return exec.Command("sh", "-c", "exit 1")
					}
					return exec.Command(name, args...)
				}
			},
			expectError: true,
			description: "Should handle when command exits with non-zero code",
		},
		{
			name: "generic command failure",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						// Default failure
						return exec.Command("false")
					}
					return exec.Command(name, args...)
				}
			},
			expectError: true,
			description: "Should handle generic command failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			var buf bytes.Buffer
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, &buf)

			// Store original execCommand to restore later
			originalExecCommand := execCommand
			defer func() {
				execCommand = originalExecCommand
			}()

			// Setup mock
			execCommand = tt.setupMock()

			// Call function
			err := CheckGitIsInstalled()

			// Verify expectations
			if tt.expectError {
				require.Error(t, err, "Expected error for test case: %s", tt.name)

				// Verify error messages are printed
				output := buf.String()
				require.Contains(t, output, "Git tool is not available",
					"Should print 'Git tool is not available' message for %s", tt.description)
				require.Contains(t, output, "Please follow install instructions at https://git-scm.com/book/en/v2/Getting-Started-Installing-Git",
					"Should print installation instructions for %s", tt.description)

				// Log the actual error for debugging
				t.Logf("Test '%s': Error = %v, Output = %s", tt.name, err, output)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestCheckGitIsInstalledCommandExecutionErrors(t *testing.T) {
	// Test specific error types that can occur during command execution
	t.Run("command execution timeout simulation", func(t *testing.T) {
		// Initialize ux.Logger for testing
		var buf bytes.Buffer
		ux.Logger = nil
		ux.NewUserLog(logging.NoLog{}, &buf)

		// Store original execCommand to restore later
		originalExecCommand := execCommand
		defer func() {
			execCommand = originalExecCommand
		}()

		// Create a command that will take time but we'll simulate failure
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
				// Return a command that sleeps briefly then fails
				return exec.Command("sh", "-c", "sleep 0.1; exit 2")
			}
			return originalExecCommand(name, args...)
		}

		// Call function
		err := CheckGitIsInstalled()

		// Verify error handling
		require.Error(t, err, "Expected error for timeout simulation")

		output := buf.String()
		require.Contains(t, output, "Git tool is not available",
			"Should print error message for timeout scenario")
	})

	t.Run("command with stderr output", func(t *testing.T) {
		// Initialize ux.Logger for testing
		var buf bytes.Buffer
		ux.Logger = nil
		ux.NewUserLog(logging.NoLog{}, &buf)

		// Store original execCommand to restore later
		originalExecCommand := execCommand
		defer func() {
			execCommand = originalExecCommand
		}()

		// Create a command that outputs to stderr and fails
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
				// Return a command that writes to stderr and exits with error
				return exec.Command("sh", "-c", "echo 'git: command not found' >&2; exit 127")
			}
			return originalExecCommand(name, args...)
		}

		// Call function
		err := CheckGitIsInstalled()

		// Verify error handling
		require.Error(t, err, "Expected error for stderr output scenario")

		output := buf.String()
		require.Contains(t, output, "Git tool is not available",
			"Should print error message for stderr scenario")
	})
}

// Integration test that uses the real CheckGitIsInstalled function
func TestCheckGitIsInstalledIntegration(t *testing.T) {
	// This test calls the real function to ensure it works
	// We can't easily mock it, but we can test that it doesn't panic
	// and that it returns some result (error or nil)
	t.Run("real function executes without panic", func(t *testing.T) {
		// Initialize ux.Logger for testing
		var buf bytes.Buffer
		ux.Logger = nil
		ux.NewUserLog(logging.NoLog{}, &buf)

		// Call the real function (no mocking)
		err := CheckGitIsInstalled()

		// We can't be sure about the result since it depends on the system
		// But we can verify it doesn't panic and returns a reasonable result
		t.Logf("CheckGitIsInstalled returned: %v", err)
		t.Logf("Output: %s", buf.String())

		// The function should either succeed (git available) or fail (git not available)
		// Both are valid outcomes, so we just verify no panic occurred
		// and that if there's an error, appropriate messages were logged
		if err != nil {
			output := buf.String()
			require.Contains(t, output, "Git tool is not available",
				"If git check fails, should print appropriate error message")
		}
	})
}

func TestBuildCustomVM(t *testing.T) {
	// Mock sidecar for testing
	mockSidecar := &models.Sidecar{
		Name:                "test-vm",
		CustomVMRepoURL:     "https://github.com/test/vm.git",
		CustomVMBranch:      "main",
		CustomVMBuildScript: buildScript,
	}

	tests := []struct {
		name            string
		setupMock       func() func(string, ...string) *exec.Cmd
		setupFS         func(t *testing.T) (string, func()) // Returns temp dir and cleanup function
		expectedError   string
		shouldCreateDir bool
	}{
		{
			name: "successful build",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						// When build script is called, create the VM binary file
						// The first argument should be the VM binary path
						if len(args) > 0 {
							vmBinaryPath := args[0]
							// Create the VM binary file with some mock content
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								// If we can't create the file, return a failing command
								return exec.Command("false")
							}
							// Make file executable after creation
							if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
								return exec.Command("false")
							}
						}
						return exec.Command("true") // Build script succeeds
					}
					return exec.Command(name, args...)
				}
			},
			setupFS:         setupTempDirWithoutVM,
			shouldCreateDir: false, // Don't pre-create VM file
		},
		{
			name: "git check fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand && len(args) > 0 && args[0] == versionFlag {
						return exec.Command("false") // Git check fails
					}
					return exec.Command("true") // Other commands succeed
				}
			},
			setupFS:       func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError: "exit status 1",
		},
		{
			name: "git init fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						if len(args) > 0 && args[0] == versionFlag {
							return exec.Command("true") // Git check succeeds
						}
						if len(args) > 0 && args[0] == initCommand {
							return exec.Command("false") // Git init fails
						}
					}
					return exec.Command("true")
				}
			},
			setupFS:         func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError:   "could not init git directory",
			shouldCreateDir: true,
		},
		{
			name: "git remote add fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						if len(args) > 0 && args[0] == versionFlag {
							return exec.Command("true") // Git check succeeds
						}
						if len(args) > 0 && args[0] == initCommand {
							return exec.Command("true") // Git init succeeds
						}
						if len(args) > 0 && args[0] == remoteCommand {
							return exec.Command("false") // Git remote add fails
						}
					}
					return exec.Command("true")
				}
			},
			setupFS:         func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError:   "could not add origin",
			shouldCreateDir: true,
		},
		{
			name: "git fetch fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						if len(args) > 0 && args[0] == versionFlag {
							return exec.Command("true") // Git check succeeds
						}
						if len(args) > 0 && args[0] == initCommand {
							return exec.Command("true") // Git init succeeds
						}
						if len(args) > 0 && args[0] == remoteCommand {
							return exec.Command("true") // Git remote add succeeds
						}
						if len(args) > 0 && args[0] == "fetch" {
							return exec.Command("false") // Git fetch fails
						}
					}
					return exec.Command("true")
				}
			},
			setupFS:         func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError:   "could not fetch git branch/commit",
			shouldCreateDir: true,
		},
		{
			name: "git checkout fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						if len(args) > 0 && args[0] == versionFlag {
							return exec.Command("true") // Git check succeeds
						}
						if len(args) > 0 && args[0] == initCommand {
							return exec.Command("true") // Git init succeeds
						}
						if len(args) > 0 && args[0] == remoteCommand {
							return exec.Command("true") // Git remote add succeeds
						}
						if len(args) > 0 && args[0] == "fetch" {
							return exec.Command("true") // Git fetch succeeds
						}
						if len(args) > 0 && args[0] == "checkout" {
							return exec.Command("false") // Git checkout fails
						}
					}
					return exec.Command("true")
				}
			},
			setupFS:         func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError:   "could not checkout git branch",
			shouldCreateDir: true,
		},
		{
			name: "build script fails",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, _ ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						return exec.Command("false") // Build script fails
					}
					return exec.Command("true")
				}
			},
			setupFS:         func(_ *testing.T) (string, func()) { return "", func() {} },
			expectedError:   "error building custom vm binary",
			shouldCreateDir: true,
		},
		{
			name: "vm binary not created after build",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					// All commands succeed but file won't be created
					return exec.Command("true")
				}
			},
			setupFS:         setupTempDirWithoutVM,
			expectedError:   "custom VM binary",
			shouldCreateDir: true,
		},
		{
			name: "vm binary not executable after build",
			setupMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						// When build script is called, create a non-executable VM binary file
						// The first argument should be the VM binary path
						if len(args) > 0 {
							vmBinaryPath := args[0]
							// Create the VM binary file with some mock content but no execute permissions
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								// If we can't create the file, return a failing command
								return exec.Command("false")
							}
							// Don't make it executable - leave as 0o600
						}
						return exec.Command("true") // Build script succeeds
					}
					return exec.Command(name, args...)
				}
			},
			setupFS:         setupTempDirWithoutVM,
			expectedError:   "not executable",
			shouldCreateDir: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, io.Discard)

			// Store original execCommand to restore later
			originalExecCommand := execCommand
			defer func() {
				execCommand = originalExecCommand
			}()

			// Setup mock
			execCommand = tt.setupMock()

			// Setup filesystem (if needed)
			tempDir, cleanup := tt.setupFS(t)
			defer cleanup()

			// Create a temporary app with mock directories
			testApp := createTestApp(t, tempDir, tt.shouldCreateDir)

			// Call the function under test
			err := BuildCustomVM(testApp, mockSidecar)

			// Verify expectations
			if tt.expectedError != "" {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				require.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
				t.Logf("Test '%s': Error = %v", tt.name, err)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestBuildCustomVMCommandSequence(t *testing.T) {
	// This test verifies that the correct sequence of commands is executed
	t.Run("correct command sequence is executed", func(t *testing.T) {
		// Initialize ux.Logger for testing
		ux.Logger = nil
		ux.NewUserLog(logging.NoLog{}, io.Discard)

		var capturedCommands [][]string

		// Store original execCommand to restore later
		originalExecCommand := execCommand
		defer func() {
			execCommand = originalExecCommand
		}()

		// Mock execCommand to capture all commands
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedCommands = append(capturedCommands, append([]string{name}, args...))

			// Special handling for build script to create VM file
			if name == buildScript && len(args) > 0 {
				vmBinaryPath := args[0]
				// Create the VM binary file with some mock content
				mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
				if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
					// If we can't create the file, return a failing command
					return exec.Command("false")
				}
				// Make file executable after creation
				if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
					return exec.Command("false")
				}
			}

			return exec.Command("true") // All commands succeed
		}

		// Setup filesystem
		tempDir, cleanup := setupTempDirWithoutVM(t)
		defer cleanup()

		// Create test app
		testApp := createTestApp(t, tempDir, false)

		mockSidecar := &models.Sidecar{
			Name:                "test-vm",
			CustomVMRepoURL:     "https://github.com/test/vm.git",
			CustomVMBranch:      "main",
			CustomVMBuildScript: buildScript,
		}

		// Call the function
		err := BuildCustomVM(testApp, mockSidecar)
		require.NoError(t, err, "BuildCustomVM should succeed")

		// Verify the correct command sequence
		expectedCommands := [][]string{
			{gitCommand, versionFlag},       // CheckGitIsInstalled
			{gitCommand, initCommand, "-q"}, // Git init
			{gitCommand, remoteCommand, "add", "origin", "https://github.com/test/vm.git"}, // Git remote add
			{gitCommand, "fetch", "--depth", "1", "origin", "main", "-q"},                  // Git fetch
			{gitCommand, "checkout", "main"},                                               // Git checkout
			{buildScript, testApp.GetCustomVMPath("test-vm")},                              // Build script
		}

		require.Equal(t, len(expectedCommands), len(capturedCommands), "Should execute correct number of commands")

		for i, expectedCmd := range expectedCommands {
			if i < len(capturedCommands) {
				require.Equal(t, expectedCmd, capturedCommands[i], "Command %d should match expected", i)
			}
		}
	})
}

// Helper functions for filesystem setup

func setupTempDirWithoutVM(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "test-vm-*")
	require.NoError(t, err, "Should create temp directory")

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func createTestApp(t *testing.T, tempDir string, shouldCreateVMFile bool) *application.Avalanche {
	// Create a proper application instance
	app := application.New()

	// Setup the app with our temp directory as base directory
	if tempDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "test-app-*")
		require.NoError(t, err, "Should create temp directory for app")
	}

	// Setup the app with minimal required components
	app.Setup(
		tempDir,          // baseDir
		logging.NoLog{},  // log
		&config.Config{}, // conf
		"test",           // version
		nil,              // prompt (not needed for BuildCustomVM)
		nil,              // downloader (not needed for BuildCustomVM)
		nil,              // cmd (not needed for BuildCustomVM)
	)

	// Create necessary directory structure
	reposDir := app.GetReposDir()
	err := os.MkdirAll(reposDir, 0o755)
	require.NoError(t, err, "Should create repos directory")

	// Create custom VM directory
	vmDir := app.GetCustomVMDir()
	err = os.MkdirAll(vmDir, 0o755)
	require.NoError(t, err, "Should create custom VM directory")

	// Create custom VM file if needed
	if shouldCreateVMFile {
		vmPath := app.GetCustomVMPath("test-vm")
		// Create a dummy executable file
		err = os.WriteFile(vmPath, []byte("#!/bin/bash\necho 'mock vm'\n"), 0o600)
		require.NoError(t, err, "Should create mock VM binary")
		// Make it executable
		err = os.Chmod(vmPath, 0o755)
		require.NoError(t, err, "Should make mock VM binary executable")
	}

	return app
}

// Helper function to create a basic test app without complex setup
func createBasicTestApp(t *testing.T) *application.Avalanche {
	app := application.New()

	// Create a temporary directory for the app
	tempDir, err := os.MkdirTemp("", "test-app-basic-*")
	require.NoError(t, err, "Should create temp directory for basic app")

	// Cleanup will be handled by the individual tests
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	// Setup the app with minimal required components
	app.Setup(
		tempDir,          // baseDir
		logging.NoLog{},  // log
		&config.Config{}, // conf
		"test",           // version
		nil,              // prompt (will be set by tests)
		nil,              // downloader (not needed for this test)
		nil,              // cmd (not needed for this test)
	)

	return app
}

func TestSetCustomVMSourceCodeFields(t *testing.T) {
	tests := []struct {
		name             string
		inputRepoURL     string
		inputBranch      string
		inputBuildScript string
		setupMocks       func(*promptsMocks.Prompter)
		expectedSidecar  *models.Sidecar
		expectedError    string
	}{
		{
			name:             "empty repo URL - all fields prompted",
			inputRepoURL:     "",
			inputBranch:      "",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/user/repo.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/user/repo.git").Return("feature-branch", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/user/repo.git", "feature-branch").Return("build.sh", nil)
			},
			expectedSidecar: &models.Sidecar{
				CustomVMRepoURL:     "https://github.com/user/repo.git",
				CustomVMBranch:      "feature-branch",
				CustomVMBuildScript: "build.sh",
			},
		},
		{
			name:             "repo URL provided but validation may fail - all fields may be prompted",
			inputRepoURL:     "https://github.com/test/vm.git",
			inputBranch:      "",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// If validation fails, it will prompt for URL first
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("develop", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "develop").Return("./scripts/build.sh", nil)
			},
			expectedSidecar: &models.Sidecar{
				CustomVMRepoURL:     "https://github.com/test/vm.git",
				CustomVMBranch:      "develop",
				CustomVMBuildScript: "./scripts/build.sh",
			},
		},
		{
			name:             "repo URL and branch provided but validation may fail - all fields may be prompted",
			inputRepoURL:     "https://github.com/test/vm.git",
			inputBranch:      "main",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// If validation fails, it will prompt for URL first, then branch, then script
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return("./build.sh", nil)
			},
			expectedSidecar: &models.Sidecar{
				CustomVMRepoURL:     "https://github.com/test/vm.git",
				CustomVMBranch:      "main",
				CustomVMBuildScript: "./build.sh",
			},
		},
		{
			name:             "build script provided, repo URL and branch prompted",
			inputRepoURL:     "",
			inputBranch:      "",
			inputBuildScript: "./scripts/build.sh",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/custom/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/custom/vm.git").Return("develop", nil)
				// Build script validation may also fail and prompt
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/custom/vm.git", "develop").Return("./scripts/build.sh", nil)
			},
			expectedSidecar: &models.Sidecar{
				CustomVMRepoURL:     "https://github.com/custom/vm.git",
				CustomVMBranch:      "develop",
				CustomVMBuildScript: "./scripts/build.sh",
			},
		},
		{
			name:             "CaptureURL fails",
			inputRepoURL:     "",
			inputBranch:      "",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("", errors.New("failed to capture URL"))
			},
			expectedError: "failed to capture URL",
		},
		{
			name:             "CaptureRepoBranch fails",
			inputRepoURL:     "https://github.com/test/vm.git",
			inputBranch:      "",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// URL validation may fail, so mock CaptureURL first
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("", errors.New("failed to capture branch"))
			},
			expectedError: "failed to capture branch",
		},
		{
			name:             "CaptureRepoFile fails",
			inputRepoURL:     "https://github.com/test/vm.git",
			inputBranch:      "main",
			inputBuildScript: "",
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// URL and branch validation may fail, so mock the fallback prompts
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return("", errors.New("failed to capture build script"))
			},
			expectedError: "failed to capture build script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, io.Discard)

			// Create mock prompter
			mockPrompter := promptsMocks.NewPrompter(t)

			// Setup mock expectations
			tt.setupMocks(mockPrompter)

			// Create test app with mock prompter
			testApp := createBasicTestApp(t)
			testApp.Prompt = mockPrompter

			// Create sidecar
			sidecar := &models.Sidecar{
				Name: "test-vm",
			}

			// Call the function under test
			err := SetCustomVMSourceCodeFields(testApp, sidecar, tt.inputRepoURL, tt.inputBranch, tt.inputBuildScript)

			// Verify expectations
			if tt.expectedError != "" {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				require.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
				t.Logf("Test '%s': Error = %v", tt.name, err)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tt.name)

				// Verify sidecar was updated correctly
				require.Equal(t, tt.expectedSidecar.CustomVMRepoURL, sidecar.CustomVMRepoURL, "CustomVMRepoURL should match")
				require.Equal(t, tt.expectedSidecar.CustomVMBranch, sidecar.CustomVMBranch, "CustomVMBranch should match")
				require.Equal(t, tt.expectedSidecar.CustomVMBuildScript, sidecar.CustomVMBuildScript, "CustomVMBuildScript should match")
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestCreateCustomSidecar(t *testing.T) {
	tests := []struct {
		name                     string
		inputSidecar             *models.Sidecar
		subnetName               string
		useRepo                  bool
		customVMRepoURL          string
		customVMBranch           string
		customVMBuildScript      string
		vmPath                   string
		tokenSymbol              string
		sovereign                bool
		setupMocks               func(*promptsMocks.Prompter)
		setupExecMock            func() func(string, ...string) *exec.Cmd
		setupProtocolVersionMock func() func(string) (int, error)
		setupAppMethods          func(*application.Avalanche)
		expectedSidecar          *models.Sidecar
		expectedError            string
	}{
		{
			name:                "successful creation with repository - all params provided",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "https://github.com/test/vm.git",
			customVMBranch:      "main",
			customVMBuildScript: buildScript,
			vmPath:              "",
			tokenSymbol:         "TEST",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Even though values are provided, validation may fail and fallback to prompting
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return(buildScript, nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						// Create VM binary when build script is called
						if len(args) > 0 {
							vmBinaryPath := args[0]
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								return exec.Command("false")
							}
							if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
								return exec.Command("false")
							}
						}
						return exec.Command("true")
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 42, nil // Mock protocol version
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedSidecar: &models.Sidecar{
				Name:                "test-vm",
				VM:                  models.CustomVM,
				Subnet:              "test-vm",
				TokenSymbol:         "TEST",
				TokenName:           "TEST Token",
				CustomVMRepoURL:     "https://github.com/test/vm.git",
				CustomVMBranch:      "main",
				CustomVMBuildScript: buildScript,
				RPCVersion:          42,
				Sovereign:           false,
			},
		},
		{
			name:                "successful creation with local binary",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "TEMP_BINARY_PLACEHOLDER", // Will be replaced in test
			tokenSymbol:         "",
			sovereign:           true,
			setupMocks: func(_ *promptsMocks.Prompter) {
				// No prompting needed - vmPath provided
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true") // Not used for local binary
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 24, nil // Mock protocol version
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedSidecar: &models.Sidecar{
				Name:                "test-vm",
				VM:                  models.CustomVM,
				Subnet:              "test-vm",
				TokenSymbol:         "",
				TokenName:           "",
				CustomVMRepoURL:     "",
				CustomVMBranch:      "",
				CustomVMBuildScript: "",
				RPCVersion:          24,
				Sovereign:           true,
			},
		},
		{
			name:                "prompt for repository vs local binary - choose repository",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "", // Empty vmPath triggers prompting
			tokenSymbol:         "PROMPT",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureList", "How do you want to set up the VM binary?", mock.AnythingOfType("[]string")).Return("Download and build from a git repository (recommended for cloud deployments)", nil)
				// Will then need repo details
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/prompted/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/prompted/vm.git").Return("develop", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/prompted/vm.git", "develop").Return("./scripts/build.sh", nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == "./scripts/build.sh" {
						// Create VM binary when build script is called
						if len(args) > 0 {
							vmBinaryPath := args[0]
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								return exec.Command("false")
							}
							if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
								return exec.Command("false")
							}
						}
						return exec.Command("true")
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 33, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedSidecar: &models.Sidecar{
				Name:                "test-vm",
				VM:                  models.CustomVM,
				Subnet:              "test-vm",
				TokenSymbol:         "PROMPT",
				TokenName:           "PROMPT Token",
				CustomVMRepoURL:     "https://github.com/prompted/vm.git",
				CustomVMBranch:      "develop",
				CustomVMBuildScript: "./scripts/build.sh",
				RPCVersion:          33,
				Sovereign:           false,
			},
		},
		{
			name:                "prompt for repository vs local binary - choose local binary",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "", // Empty vmPath triggers prompting
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureList", "How do you want to set up the VM binary?", mock.AnythingOfType("[]string")).Return("I already have a VM binary (local network deployments only)", nil)
				mockPrompter.On("CaptureExistingFilepath", "Enter path to VM binary").Return("TEMP_BINARY_PLACEHOLDER", nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true") // Not used for local binary
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 55, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for CopyVMBinary
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedSidecar: &models.Sidecar{
				Name:                "test-vm",
				VM:                  models.CustomVM,
				Subnet:              "test-vm",
				TokenSymbol:         "",
				TokenName:           "",
				CustomVMRepoURL:     "",
				CustomVMBranch:      "",
				CustomVMBuildScript: "",
				RPCVersion:          55,
				Sovereign:           false,
			},
		},
		{
			name: "existing sidecar provided",
			inputSidecar: &models.Sidecar{
				Name:        "existing-vm",
				VM:          models.SubnetEvm, // Will be overridden
				TokenSymbol: "EXISTING",
			},
			subnetName:          "new-name", // Will override sidecar name
			useRepo:             true,
			customVMRepoURL:     "https://github.com/existing/vm.git",
			customVMBranch:      "feature",
			customVMBuildScript: "./build_existing.sh",
			vmPath:              "",
			tokenSymbol:         "NEW", // Will override existing token
			sovereign:           true,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Even though values are provided, validation may fail and fallback to prompting
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/existing/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/existing/vm.git").Return("feature", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/existing/vm.git", "feature").Return("./build_existing.sh", nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == "./build_existing.sh" {
						// Create VM binary when build script is called
						if len(args) > 0 {
							vmBinaryPath := args[0]
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								return exec.Command("false")
							}
							if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
								return exec.Command("false")
							}
						}
						return exec.Command("true")
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 77, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedSidecar: &models.Sidecar{
				Name:                "new-name",      // Updated from parameter
				VM:                  models.CustomVM, // Updated from CustomVM
				Subnet:              "new-name",      // Updated from parameter
				TokenSymbol:         "NEW",           // Updated from parameter
				TokenName:           "NEW Token",     // Updated from parameter
				CustomVMRepoURL:     "https://github.com/existing/vm.git",
				CustomVMBranch:      "feature",
				CustomVMBuildScript: "./build_existing.sh",
				RPCVersion:          77,
				Sovereign:           true,
			},
		},
		{
			name:                "CaptureList fails",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "", // Empty vmPath triggers prompting
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureList", "How do you want to set up the VM binary?", mock.AnythingOfType("[]string")).Return("", errors.New("failed to capture choice"))
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true")
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(_ *application.Avalanche) {},
			expectedError:   "failed to capture choice",
		},
		{
			name:                "CaptureExistingFilepath fails",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "", // Empty vmPath triggers prompting
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				mockPrompter.On("CaptureList", "How do you want to set up the VM binary?", mock.AnythingOfType("[]string")).Return("I already have a VM binary (local network deployments only)", nil)
				mockPrompter.On("CaptureExistingFilepath", "Enter path to VM binary").Return("", errors.New("failed to capture filepath"))
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true")
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(_ *application.Avalanche) {},
			expectedError:   "failed to capture filepath",
		},
		{
			name:                "GetVMBinaryProtocolVersion fails",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "https://github.com/test/vm.git",
			customVMBranch:      "main",
			customVMBuildScript: buildScript,
			vmPath:              "",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Even though values are provided, validation may fail and fallback to prompting
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return(buildScript, nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true")
					}
					if name == buildScript {
						// Create VM binary when build script is called
						if len(args) > 0 {
							vmBinaryPath := args[0]
							mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
							if err := os.WriteFile(vmBinaryPath, mockVMContent, 0o600); err != nil {
								return exec.Command("false")
							}
							if err := os.Chmod(vmBinaryPath, 0o755); err != nil {
								return exec.Command("false")
							}
						}
						return exec.Command("true")
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, errors.New("failed to get protocol version")
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedError: "unable to get RPC version: failed to get protocol version",
		},
		{
			name:                "CopyVMBinary fails - source file doesn't exist",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "/nonexistent/path/to/vm-binary",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(_ *promptsMocks.Prompter) {
				// No prompting needed - vmPath provided
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true") // Not used for local binary
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the VM directory structure
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedError: "no such file or directory",
		},
		{
			name:                "CopyVMBinary fails - destination directory read-only",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             false,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "TEMP_BINARY_PLACEHOLDER", // Will be replaced in test
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(_ *promptsMocks.Prompter) {
				// No prompting needed - vmPath provided
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true") // Not used for local binary
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the VM directory structure but make it read-only
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
				// Make directory read-only to simulate write failure
				if err := os.Chmod(vmDir, 0o444); err != nil {
					panic(err)
				}
			},
			expectedError: "permission denied",
		},
		{
			name:                "SetCustomVMSourceCodeFields fails",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "",
			customVMBranch:      "",
			customVMBuildScript: "",
			vmPath:              "",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Make CaptureURL fail to simulate SetCustomVMSourceCodeFields failure
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("", errors.New("failed to capture repository URL"))
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(_ string, _ ...string) *exec.Cmd {
					return exec.Command("true")
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(_ *application.Avalanche) {},
			expectedError:   "failed to capture repository URL",
		},
		{
			name:                "BuildCustomVM fails - git init failure",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "https://github.com/test/vm.git",
			customVMBranch:      "main",
			customVMBuildScript: buildScript,
			vmPath:              "",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Provide valid inputs for SetCustomVMSourceCodeFields
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return(buildScript, nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						if len(args) > 0 && args[0] == versionFlag {
							return exec.Command("true") // Git check succeeds
						}
						if len(args) > 0 && args[0] == initCommand {
							return exec.Command("false") // Git init fails
						}
					}
					return exec.Command("true")
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedError: "could not init git directory",
		},
		{
			name:                "BuildCustomVM fails - build script execution failure",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "https://github.com/test/vm.git",
			customVMBranch:      "main",
			customVMBuildScript: buildScript,
			vmPath:              "",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Provide valid inputs for SetCustomVMSourceCodeFields
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return(buildScript, nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						return exec.Command("false") // Build script fails
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedError: "error building custom vm binary using script",
		},
		{
			name:                "BuildCustomVM fails - VM binary not created after build",
			inputSidecar:        nil,
			subnetName:          "test-vm",
			useRepo:             true,
			customVMRepoURL:     "https://github.com/test/vm.git",
			customVMBranch:      "main",
			customVMBuildScript: buildScript,
			vmPath:              "",
			tokenSymbol:         "",
			sovereign:           false,
			setupMocks: func(mockPrompter *promptsMocks.Prompter) {
				// Provide valid inputs for SetCustomVMSourceCodeFields
				mockPrompter.On("CaptureURL", "Source code repository URL", true).Return("https://github.com/test/vm.git", nil)
				mockPrompter.On("CaptureRepoBranch", "Branch", "https://github.com/test/vm.git").Return("main", nil)
				mockPrompter.On("CaptureRepoFile", "Build script", "https://github.com/test/vm.git", "main").Return(buildScript, nil)
			},
			setupExecMock: func() func(string, ...string) *exec.Cmd {
				return func(name string, args ...string) *exec.Cmd {
					if name == gitCommand {
						return exec.Command("true") // All git commands succeed
					}
					if name == buildScript {
						// Build script succeeds but doesn't create the VM binary
						return exec.Command("true")
					}
					return exec.Command(name, args...)
				}
			},
			setupProtocolVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil
				}
			},
			setupAppMethods: func(app *application.Avalanche) {
				// Create the necessary directory structure for the build process
				reposDir := app.GetReposDir()
				if err := os.MkdirAll(reposDir, 0o755); err != nil {
					panic(err)
				}
				vmDir := app.GetCustomVMDir()
				if err := os.MkdirAll(vmDir, 0o755); err != nil {
					panic(err)
				}
			},
			expectedError: "custom VM binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize ux.Logger for testing
			ux.Logger = nil
			ux.NewUserLog(logging.NoLog{}, io.Discard)

			// Store original functions to restore later
			originalExecCommand := execCommand
			originalGetVMBinaryProtocolVersion := getVMBinaryProtocolVersion
			defer func() {
				execCommand = originalExecCommand
				getVMBinaryProtocolVersion = originalGetVMBinaryProtocolVersion
			}()

			// Create temporary binary file if needed
			tempBinaryPath := ""
			needsTempBinary := tt.vmPath == "TEMP_BINARY_PLACEHOLDER" ||
				strings.Contains(tt.name, "local binary")
			if needsTempBinary {
				tempDir := t.TempDir()
				tempBinaryPath = filepath.Join(tempDir, "vm-binary")
				mockVMContent := []byte("#!/bin/bash\necho 'Mock VM binary'\n")
				if err := os.WriteFile(tempBinaryPath, mockVMContent, 0o600); err != nil {
					t.Fatalf("Failed to create temp binary: %v", err)
				}
				// Make it executable after creation
				if err := os.Chmod(tempBinaryPath, 0o755); err != nil {
					t.Fatalf("Failed to create temp binary: %v", err)
				}
			}

			// Setup mocks
			execCommand = tt.setupExecMock()
			getVMBinaryProtocolVersion = tt.setupProtocolVersionMock()

			// Create test app
			testApp := createBasicTestApp(t)

			// Setup prompter mock
			mockPrompter := &promptsMocks.Prompter{}
			// Create a custom setup function that can use the temp binary path
			setupMocksFunc := tt.setupMocks
			if tempBinaryPath != "" {
				// For tests that need temp binary, we need to modify the mock setup
				setupMocksFunc = func(mp *promptsMocks.Prompter) {
					// Call original setup first, but ignore any calls with placeholder
					if strings.Contains(tt.name, "prompt for repository vs local binary - choose local binary") {
						mp.On("CaptureList", "How do you want to set up the VM binary?", mock.AnythingOfType("[]string")).Return("I already have a VM binary (local network deployments only)", nil)
						mp.On("CaptureExistingFilepath", "Enter path to VM binary").Return(tempBinaryPath, nil)
					} else {
						// For other cases, use original setup
						tt.setupMocks(mp)
					}
				}
			}
			setupMocksFunc(mockPrompter)
			testApp.Prompt = mockPrompter

			// Setup app method mocks
			tt.setupAppMethods(testApp)

			// Replace placeholder in vmPath if needed
			vmPath := tt.vmPath
			if vmPath == "TEMP_BINARY_PLACEHOLDER" {
				vmPath = tempBinaryPath
			}

			// Call the function under test
			result, err := CreateCustomSidecar(
				tt.inputSidecar,
				testApp,
				tt.subnetName,
				tt.useRepo,
				tt.customVMRepoURL,
				tt.customVMBranch,
				tt.customVMBuildScript,
				vmPath,
				tt.tokenSymbol,
				tt.sovereign,
			)

			// Verify expectations
			if tt.expectedError != "" {
				require.Error(t, err, "Expected error for test case: %s", tt.name)
				require.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
				require.Nil(t, result, "Result should be nil when error occurs")
				t.Logf("Test '%s': Error = %v", tt.name, err)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tt.name)
				require.NotNil(t, result, "Result should not be nil")

				// Compare expected fields
				require.Equal(t, tt.expectedSidecar.Name, result.Name, "Name should match")
				require.Equal(t, tt.expectedSidecar.VM, result.VM, "VM should match")
				require.Equal(t, tt.expectedSidecar.Subnet, result.Subnet, "Subnet should match")
				require.Equal(t, tt.expectedSidecar.TokenSymbol, result.TokenSymbol, "TokenSymbol should match")
				require.Equal(t, tt.expectedSidecar.TokenName, result.TokenName, "TokenName should match")
				require.Equal(t, tt.expectedSidecar.CustomVMRepoURL, result.CustomVMRepoURL, "CustomVMRepoURL should match")
				require.Equal(t, tt.expectedSidecar.CustomVMBranch, result.CustomVMBranch, "CustomVMBranch should match")
				require.Equal(t, tt.expectedSidecar.CustomVMBuildScript, result.CustomVMBuildScript, "CustomVMBuildScript should match")
				require.Equal(t, tt.expectedSidecar.RPCVersion, result.RPCVersion, "RPCVersion should match")
				require.Equal(t, tt.expectedSidecar.Sovereign, result.Sovereign, "Sovereign should match")
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

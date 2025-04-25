// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestValidateRPC(t *testing.T) {
	tests := []struct {
		name           string
		rpcURL         string
		cmdName        string
		args           []string
		promptURL      string
		promptError    error
		wantError      bool
		shouldPrompt   bool
		expectedRPCURL string
	}{
		// URL validation test cases
		{
			name:           "Valid HTTP URL",
			rpcURL:         "http://localhost:9650",
			cmdName:        "testcmd",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "http://localhost:9650",
		},
		{
			name:           "Valid HTTPS URL",
			rpcURL:         "https://api.avax.network:9650",
			cmdName:        "testcmd",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "https://api.avax.network:9650",
		},
		{
			name:           "Invalid URL - no protocol",
			rpcURL:         "localhost:9650",
			cmdName:        "testcmd",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "localhost:9650",
		},
		{
			name:           "Invalid URL - malformed",
			rpcURL:         "http://invalid url",
			cmdName:        "testcmd",
			args:           []string{},
			wantError:      true,
			shouldPrompt:   false,
			expectedRPCURL: "http://invalid url",
		},
		// Command-specific test cases
		{
			name:           "addValidator command with empty URL - valid prompt response",
			rpcURL:         "",
			cmdName:        "addValidator",
			args:           []string{},
			promptURL:      "https://api.avax.network:9650",
			promptError:    nil,
			wantError:      false,
			shouldPrompt:   true,
			expectedRPCURL: "https://api.avax.network:9650",
		},
		{
			name:           "addValidator command with empty URL - invalid prompt response",
			rpcURL:         "",
			cmdName:        "addValidator",
			args:           []string{},
			promptURL:      "invalid-url",
			promptError:    fmt.Errorf("invalid URL"),
			wantError:      true,
			shouldPrompt:   true,
			expectedRPCURL: "invalid-url",
		},
		{
			name:           "addValidator command with args",
			rpcURL:         "",
			cmdName:        "addValidator",
			args:           []string{"arg1"},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "",
		},
		{
			name:           "addValidator command with valid URL",
			rpcURL:         "https://api.avax.network:9650",
			cmdName:        "addValidator",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "https://api.avax.network:9650",
		},
		{
			name:           "non-addValidator command with empty URL",
			rpcURL:         "",
			cmdName:        "otherCommand",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "",
		},
		{
			name:           "non-addValidator command with valid URL",
			rpcURL:         "https://api.avax.network:9650",
			cmdName:        "otherCommand",
			args:           []string{},
			wantError:      false,
			shouldPrompt:   false,
			expectedRPCURL: "https://api.avax.network:9650",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Create and configure mock prompter if needed
			mockPrompter := mocks.Prompter{}
			if tt.cmdName == "addValidator" && len(tt.args) == 0 && tt.rpcURL == "" {
				mockPrompter.On("CaptureURL", "What is the RPC endpoint?", false).
					Return(tt.promptURL, tt.promptError)
			}

			app := &application.Avalanche{
				Prompt: &mockPrompter,
			}
			cmd := &cobra.Command{Use: tt.cmdName}

			rpcValue := tt.rpcURL
			err := ValidateRPC(app, &rpcValue, cmd, tt.args)

			// Check error expectation
			if tt.wantError {
				require.Error(err)
			} else {
				require.NoError(err)
			}

			// Verify prompt expectations
			mockPrompter.AssertExpectations(t)

			// Verify prompt calls
			if tt.shouldPrompt {
				mockPrompter.AssertCalled(t, "CaptureURL", "What is the RPC endpoint?", false)
			} else {
				mockPrompter.AssertNotCalled(t, "CaptureURL", "What is the RPC endpoint?", false)
			}

			// Verify final RPC URL value
			require.Equal(tt.expectedRPCURL, rpcValue, "RPC URL value mismatch")
		})
	}
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/version"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestPublisher(string, string, string) subnet.Publisher {
	mockPub := &mocks.Publisher{}
	mockPub.On("GetRepo").Return(&git.Repository{}, nil)
	mockPub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockPub
}

// TestPublisher allows unit testing of the **normal** flow for publishing
func TestPublisher(t *testing.T) {
	assert, mockPrompt := setupTestEnv(t)

	// capture string for a repo alias...
	mockPrompt.On("CaptureString", mock.Anything).Return("testAlias", nil).Once()
	// then the repo URL...
	mockPrompt.On("CaptureString", mock.Anything).Return("https://localhost:12345", nil).Once()
	// always provide an irrelevant response when empty is allowed...
	mockPrompt.On("CaptureEmpty", mock.Anything, mock.Anything).Return("irrelevant", nil)
	// on the maintainers, return some array
	mockPrompt.On("CaptureListDecision", mockPrompt, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]any{"dummy", "stuff"}, false, nil)
	retVer, err := version.Parse("v0.9.99")
	assert.NoError(err)
	// finally return a semantic version
	mockPrompt.On("CaptureSemanticVersion", mock.Anything).Return(retVer, nil)

	sc := &models.Sidecar{}
	err = doPublish(sc, "testSubnet", newTestPublisher)
	assert.NoError(err)
}

func setupTestEnv(t *testing.T) (*assert.Assertions, *mocks.Prompter) {
	assert := assert.New(t)
	testDir := t.TempDir()
	err := os.Mkdir(filepath.Join(testDir, "repos"), 0o755)
	assert.NoError(err)
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	app = &application.Avalanche{}
	mockPrompt := mocks.NewPrompter(t)
	app.Setup(testDir, logging.NoLog{}, config.New(), mockPrompt)

	return assert, mockPrompt
}

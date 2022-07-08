package vm

import (
	"errors"
	"io"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testToken = "TEST"

func setupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func Test_getChainId(t *testing.T) {
	assert := setupTest(t)
	app := application.New()
	mockPrompt := &mocks.Prompter{}
	app.Prompt = mockPrompt

	mockPrompt.On("CaptureString", mock.Anything).Return(testToken, nil)

	token, err := getTokenName(app)
	assert.NoError(err)
	assert.Equal(testToken, token)
}

func Test_getChainId_Err(t *testing.T) {
	assert := setupTest(t)
	app := application.New()
	mockPrompt := &mocks.Prompter{}
	app.Prompt = mockPrompt

	testErr := errors.New("Bad prompt")
	mockPrompt.On("CaptureString", mock.Anything).Return("", testErr)

	_, err := getTokenName(app)
	assert.ErrorIs(testErr, err)
}

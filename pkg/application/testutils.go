package application

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
)

func NewTestApp(t *testing.T) *Avalanche {
	tempDir := t.TempDir()
	return &Avalanche{
		baseDir: tempDir,
		Log:     logging.NoLog{},
	}
}

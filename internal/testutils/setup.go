// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testutils

import (
	"io"
	"os"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
)

func SetupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func SetupTempTestDir() (string, error) {
	return os.MkdirTemp(os.TempDir(), "cli-test")
}

func CleanTempTestDir(testDir string) {
	os.RemoveAll(testDir)
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"os"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
)

func TestCleanBins(t *testing.T) {
	assert := assert.New(t)
	ux.NewUserLog(logging.NoLog{}, os.Stdout)
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "bin-test")
	assert.NoError(err)
	f2, err := os.CreateTemp(dir, "another-test")
	assert.NoError(err)
	cleanBins(dir)
	assert.NoFileExists(f.Name())
	assert.NoFileExists(f2.Name())
	assert.NoDirExists(dir)
}

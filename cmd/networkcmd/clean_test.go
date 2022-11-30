// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"os"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

func TestCleanBins(t *testing.T) {
	require := require.New(t)
	ux.NewUserLog(logging.NoLog{}, os.Stdout)
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "bin-test")
	require.NoError(err)
	f2, err := os.CreateTemp(dir, "another-test")
	require.NoError(err)
	cleanBins(dir)
	require.NoFileExists(f.Name())
	require.NoFileExists(f2.Name())
	require.NoDirExists(dir)
}

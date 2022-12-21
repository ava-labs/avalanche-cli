// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/stretchr/testify/require"
)

func TestSetupAPM(t *testing.T) {
	require := require.New(t)
	testDir := t.TempDir()
	app := newTestApp(t, testDir)

	err := os.MkdirAll(filepath.Dir(app.GetAPMLog()), constants.DefaultPerms755)
	require.NoError(err)

	err = SetupApm(app, testDir)
	require.NoError(err)
	require.NotEqual(nil, app.Apm)
	require.Equal(testDir, app.ApmDir)
}

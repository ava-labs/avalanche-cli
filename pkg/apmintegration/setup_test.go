// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	assert := require.New(t)
	testDir, err := testutils.SetupTempTestDir()
	assert.NoError(err)
	defer testutils.CleanTempTestDir(testDir)

	app := newTestApp(t, testDir)

	err = os.MkdirAll(filepath.Dir(app.GetAPMLog()), constants.DefaultPerms755)
	assert.NoError(err)

	err = SetupApm(app, testDir)
	assert.NoError(err)
	assert.NotEqual(nil, app.Apm)
	assert.Equal(testDir, app.ApmDir)
}

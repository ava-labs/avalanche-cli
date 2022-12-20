// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package migrations

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

func TestTopLevelFilesMigration(t *testing.T) {
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	require := require.New(t)
	testDir := t.TempDir()

	app := &application.Avalanche{}
	app.Setup(testDir, logging.NoLog{}, config.New(), prompts.NewPrompter(), application.NewDownloader())

	testSC1 := &models.Sidecar{
		Name: "test1",
	}
	testSC2 := &models.Sidecar{
		Name: "test2",
	}
	testSC3 := &models.Sidecar{
		Name: "test3",
	}

	// can't use app.CreateSidecar as that will already write into the new structure
	// create files manually
	cars := []*models.Sidecar{testSC1, testSC2, testSC3}
	for _, c := range cars {
		bytesCar, err := json.Marshal(c)
		require.NoError(err)
		scFileName := filepath.Join(app.GetBaseDir(), c.Name+constants.SidecarSuffix)
		err = os.WriteFile(scFileName, bytesCar, constants.DefaultPerms755)
		require.NoError(err)
		// double check file is there, not really necessary
		_, err = os.Stat(scFileName)
		require.NoError(err)
	}

	// we'll use just one genesis file
	genesisTestFile := filepath.Join(app.GetBaseDir(), testSC2.Name+constants.GenesisSuffix)
	err := os.WriteFile(genesisTestFile, []byte("bogus"), constants.DefaultPerms755)
	require.NoError(err)
	// double check file is there, not really necessary
	_, err = os.Stat(genesisTestFile)
	require.NoError(err)

	runner := migrationRunner{
		showMsg: true,
		running: false,
		migrations: map[int]migrationFunc{
			0: migrateTopLevelFiles,
		},
	}
	// run the migration
	err = runner.run(app)
	require.NoError(err)

	// make sure all the new files have been created and the old ones don't exist anymore
	d, err := os.Stat(filepath.Join(app.GetBaseDir(), constants.SubnetDir))
	require.NoError(err)
	require.True(d.IsDir())
	for _, c := range cars {
		d, err = os.Stat(filepath.Join(app.GetBaseDir(), constants.SubnetDir, c.Name))
		require.NoError(err)
		require.True(d.IsDir())
		oldSCFileName := filepath.Join(app.GetBaseDir(), c.Name+constants.SidecarSuffix)
		_, err = os.Stat(oldSCFileName)
		require.Error(err)
		newFile := filepath.Join(app.GetSubnetDir(), c.Name, constants.SidecarFileName)
		_, err = os.Stat(newFile)
		require.NoError(err)
	}
	oldGenesis := filepath.Join(app.GetBaseDir(), testSC2.Name+constants.GenesisSuffix)
	_, err = os.Stat(oldGenesis)
	require.Error(err)
	newFile := filepath.Join(app.GetSubnetDir(), testSC2.Name, constants.GenesisFileName)
	_, err = os.Stat(newFile)
	require.NoError(err)
}

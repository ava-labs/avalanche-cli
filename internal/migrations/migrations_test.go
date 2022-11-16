// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package migrations

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
)

func TestRunMigrations(t *testing.T) {
	buffer := make([]byte, 0, 100)
	bufWriter := bytes.NewBuffer(buffer)
	ux.NewUserLog(logging.NoLog{}, bufWriter)
	assert := assert.New(t)
	testDir := t.TempDir()

	app := &application.Avalanche{}
	app.Setup(testDir, logging.NoLog{}, config.New(), prompts.NewPrompter())

	type migTest struct {
		migs           map[int]migrationFunc
		name           string
		shouldErr      bool
		expectedOutput string
	}

	expectedIfRan := runMessage + "\n" + endMessage + "\n"
	expectedIfFailed := runMessage + "\n" + failedEndMessage + "\n"

	tests := []migTest{
		{
			name:           "no migrations",
			shouldErr:      false,
			migs:           map[int]migrationFunc{},
			expectedOutput: "",
		},
		{
			name:      "migration fail",
			shouldErr: true,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					return errors.New("bogus fail")
				},
			},
			expectedOutput: "",
		},
		{
			name:      "1 mig, apply",
			shouldErr: false,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
			},
			expectedOutput: expectedIfRan,
		},
		{
			name:      "2 mig, apply both",
			shouldErr: false,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
				1: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
			},
			expectedOutput: expectedIfRan,
		},
		{
			name:      "2 mig, apply 1",
			shouldErr: false,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					return nil
				},
				1: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
			},
			expectedOutput: expectedIfRan,
		},
		{
			name:      "2 mig, first one fails",
			shouldErr: true,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					return errors.New("bogus fail")
				},
				1: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
			},
			expectedOutput: "",
		},
		{
			name:      "2 mig, apply 1, second one fails",
			shouldErr: true,
			migs: map[int]migrationFunc{
				0: func(app *application.Avalanche, r *migrationRunner) error {
					r.printMigrationMessage()
					return nil
				},
				1: func(app *application.Avalanche, r *migrationRunner) error {
					return errors.New("bogus fail")
				},
			},
			expectedOutput: expectedIfFailed,
		},
	}

	for _, tt := range tests {
		// reset the buffer on each run to match expected output
		bufWriter.Reset()
		runner := &migrationRunner{
			showMsg:    true,
			running:    false,
			migrations: tt.migs,
		}
		err := runner.run(app)
		if tt.shouldErr {
			assert.Error(err)
		} else {
			assert.NoError(err)
		}
		assert.Equal(tt.expectedOutput, bufWriter.String())
	}
}

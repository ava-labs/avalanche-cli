// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"go.uber.org/zap"
)

type LastActions struct {
	LastSkipCheck time.Time
	LastUpdated   time.Time
}

func (app *Avalanche) WriteLastActionsFile(acts *LastActions) {
	bLastActs, err := json.Marshal(&acts)
	if err != nil {
		app.Log.Warn("failed to marshal lastActions! This is non-critical but is logged", zap.Error(err))
		return
	}
	if err := os.WriteFile(
		filepath.Join(app.GetBaseDir(), constants.LastFileName),
		bLastActs,
		constants.DefaultPerms755); err != nil {
		app.Log.Warn("failed to create the last-actions file! This is non-critical but is logged", zap.Error(err))
	}
}

func (app *Avalanche) ReadLastActionsFile() (*LastActions, error) {
	var lastActs *LastActions
	fileBytes, err := os.ReadFile(filepath.Join(app.GetBaseDir(), constants.LastFileName))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fileBytes, &lastActs); err != nil {
		app.Log.Warn("failed to unmarshal lastActions! This is non-critical but is logged", zap.Error(err))
		return nil, nil
	}
	return lastActs, nil
}

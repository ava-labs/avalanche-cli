// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

type ExecutingLocalnetMeta struct {
	NetworkDir string `json:"networkDir"`
}

func executingLocalnetMetaPath(app *application.Avalanche) string {
	return filepath.Join(app.GetBaseDir(), constants.ExecutingLocalnetMetaFile)
}

func ExecutingLocalnetMetaExists(
	app *application.Avalanche,
) bool {
	return utils.FileExists(executingLocalnetMetaPath(app))
}

func GetExecutingLocalnetMeta(
	app *application.Avalanche,
) (*ExecutingLocalnetMeta, error) {
	path := executingLocalnetMetaPath(app)
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed reading executing localnet meta file at %s: %w", path, err)
	}
	var meta *ExecutingLocalnetMeta
	if err := json.Unmarshal(bs, meta); err != nil {
		return nil, fmt.Errorf("failed unmarshalling executing localnet meta file at %s: %w", path, err)
	}
	return meta, nil
}

func SaveExecutingLocalnetMeta(
	app *application.Avalanche,
	networkDir string,
) error {
	meta := ExecutingLocalnetMeta{
		NetworkDir: networkDir,
	}
	bs, err := json.Marshal(&meta)
	if err != nil {
		return err
	}
	path := executingLocalnetMetaPath(app)
	if err := os.WriteFile(path, bs, constants.WriteReadUserOnlyPerms); err != nil {
		return fmt.Errorf("could not write executing localnet meta file %s: %w", path, err)
	}
	return nil
}

func RemoveExecutingLocalnetMeta(
	app *application.Avalanche,
) error {
	return os.RemoveAll(executingLocalnetMetaPath(app))
}

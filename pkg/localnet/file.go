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

type Info struct {
	CurrentLocalNetworkDir string `json:"currentLocalNetworkDir"`
}

func infoPath(app *application.Avalanche) string {
	return filepath.Join(app.GetBaseDir(), constants.LocalNetworksFile)
}

func InfoExists(
	app *application.Avalanche,
) bool {
	return utils.FileExists(infoPath(app))
}

func ReadInfo(
	app *application.Avalanche,
) (string, error) {
	path := infoPath(app)
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed reading local networks info file at %s: %w", path, err)
	}
	var info Info
	if err := json.Unmarshal(bs, &info); err != nil {
		return "", fmt.Errorf("failed unmarshalling local networks info file at %s: %w", path, err)
	}
	return info.CurrentLocalNetworkDir, nil
}

func SaveInfo(
	app *application.Avalanche,
	currentLocalNetworkDir string,
) error {
	info := Info{
		CurrentLocalNetworkDir: currentLocalNetworkDir,
	}
	bs, err := json.Marshal(&info)
	if err != nil {
		return err
	}
	if err := os.WriteFile(infoPath(app), bs, constants.WriteReadUserOnlyPerms); err != nil {
		return fmt.Errorf("could not write local newtorks info to file: %w", err)
	}
	return nil
}

func RemoveInfo(
	app *application.Avalanche,
) error {
	return os.RemoveAll(infoPath(app))
}

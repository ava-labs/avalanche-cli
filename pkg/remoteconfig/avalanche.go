// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import (
	"bytes"
	"html/template"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

type AvalancheConfigInputs struct {
	HTTPHost         string
	APIAdminEnabled  bool
	IndexEnabled     bool
	NetworkID        string
	DBDir            string
	LogDir           string
	PublicIP         string
	StateSyncEnabled bool
	PruningEnabled   bool
}

func DefaultCliAvalancheConfig(publicIP string, networkID string) AvalancheConfigInputs {
	return AvalancheConfigInputs{
		HTTPHost:         "127.0.0.1",
		NetworkID:        networkID,
		DBDir:            "/.avalanchego/db/",
		LogDir:           "/.avalanchego/logs/",
		PublicIP:         publicIP,
		StateSyncEnabled: true,
		PruningEnabled:   false,
	}
}

func RenderAvalancheTemplate(templateName string, config AvalancheConfigInputs) ([]byte, error) {
	templateBytes, err := templates.ReadFile(templateName)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("config").Parse(string(templateBytes))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func RenderAvalancheNodeConfig(config AvalancheConfigInputs) ([]byte, error) {
	if output, err := RenderAvalancheTemplate("templates/avalanche-node.tmpl", config); err != nil {
		return nil, err
	} else {
		return output, nil
	}
}

func RenderAvalancheCChainConfig(config AvalancheConfigInputs) ([]byte, error) {
	if output, err := RenderAvalancheTemplate("templates/avalanche-cchain.tmpl", config); err != nil {
		return nil, err
	} else {
		return output, nil
	}
}

func GetRemoteAvalancheNodeConfig() string {
	return filepath.Join(constants.CloudNodeConfigPath, "node.json")
}

func GetRemoteAvalancheCChainConfig() string {
	return filepath.Join(constants.CloudNodeConfigPath, "chains", "C", "config.json")
}

func AvalancheFolderToCreate() []string {
	return []string{
		"/home/ubuntu/.avalanchego/db",
		"/home/ubuntu/.avalanchego/logs",
		"/home/ubuntu/.avalanchego/configs",
		"/home/ubuntu/.avalanchego/configs/chains/C",
		"/home/ubuntu/.avalanchego/staking",
		"/home/ubuntu/.avalanchego/plugins",
		"/home/ubuntu/.avalanche-cli/services/awm-relayer",
	}
}

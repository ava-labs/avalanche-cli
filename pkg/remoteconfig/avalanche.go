// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import (
	"bytes"
	"html/template"
)

type AvalancheConfigInputs struct {
	HttpHost         string
	ApiAdminEnabled  bool
	IndexEnabled     bool
	NetworkID        string
	DbDir            string
	LogDir           string
	PublicIP         string
	StateSyncEnabled bool
	PruningEnabled   bool
}

func DefaultCliAvalancheConfig(publicIP string, networkID string) AvalancheConfigInputs {
	return AvalancheConfigInputs{
		HttpHost:         "0.0.0.0",
		NetworkID:        networkID,
		DbDir:            "/home/ubuntu/.avalanchego/db/",
		LogDir:           "/home/ubuntu/.avalanchego/logs/",
		PublicIP:         publicIP,
		StateSyncEnabled: true,
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

func AvalancheFolderToCreate() []string {
	return []string{
		"~/.avalanchego/db",
		"~/.avalanchego/logs",
		"~/.avalanchego/configs",
		"~/.avalanchego/configs/chains/C",
		"~/.avalanchego/staking",
	}
}

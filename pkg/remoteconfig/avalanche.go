// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

type AvalancheConfigInputs struct {
	HTTPHost                   string
	APIAdminEnabled            bool
	IndexEnabled               bool
	NetworkID                  string
	DBDir                      string
	LogDir                     string
	PublicIP                   string
	StateSyncEnabled           bool
	PruningEnabled             bool
	Aliases                    []string
	BlockChainID               string
	TrackSubnets               string
	BootstrapIDs               string
	BootstrapIPs               string
	GenesisPath                string
	UpgradePath                string
	ProposerVMUseCurrentHeight bool
}

func PrepareAvalancheConfig(publicIP string, networkID string, subnets []string) AvalancheConfigInputs {
	return AvalancheConfigInputs{
		HTTPHost:                   "127.0.0.1",
		NetworkID:                  networkID,
		DBDir:                      "/.avalanchego/db/",
		LogDir:                     "/.avalanchego/logs/",
		PublicIP:                   publicIP,
		StateSyncEnabled:           true,
		PruningEnabled:             false,
		TrackSubnets:               strings.Join(subnets, ","),
		Aliases:                    nil,
		BlockChainID:               "",
		ProposerVMUseCurrentHeight: true,
	}
}

func RenderAvalancheTemplate(templateName string, config AvalancheConfigInputs) ([]byte, error) {
	templateBytes, err := templates.ReadFile(templateName)
	if err != nil {
		return nil, err
	}
	helperFuncs := template.FuncMap{
		"join": strings.Join,
	}
	tmpl, err := template.New("config").Funcs(helperFuncs).Parse(string(templateBytes))
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

func RenderAvalancheAliasesConfig(config AvalancheConfigInputs) ([]byte, error) {
	if output, err := RenderAvalancheTemplate("templates/avalanche-aliases.tmpl", config); err != nil {
		return nil, err
	} else {
		return output, nil
	}
}

func GetRemoteAvalancheNodeConfig() string {
	return filepath.Join(constants.CloudNodeConfigPath, constants.NodeFileName)
}

func GetRemoteAvalancheCChainConfig() string {
	return filepath.Join(constants.CloudNodeConfigPath, "chains", "C", "config.json")
}

func GetRemoteAvalancheGenesis() string {
	return filepath.Join(constants.CloudNodeConfigPath, constants.GenesisFileName)
}

func GetRemoteAvalancheAliasesConfig() string {
	return filepath.Join(constants.CloudNodeConfigPath, "chains", constants.AliasesFileName)
}

func AvalancheFolderToCreate() []string {
	return []string{
		"/home/ubuntu/.avalanchego/db",
		"/home/ubuntu/.avalanchego/logs",
		"/home/ubuntu/.avalanchego/configs",
		"/home/ubuntu/.avalanchego/configs/subnets/",
		"/home/ubuntu/.avalanchego/configs/chains/C",
		"/home/ubuntu/.avalanchego/staking",
		"/home/ubuntu/.avalanchego/plugins",
		"/home/ubuntu/.avalanche-cli/services/awm-relayer",
	}
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package monitoring

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

type configInputs struct {
	AvalancheGoPorts string
	MachinePorts     string
}

//go:embed dashboards/*
var dashboards embed.FS

//go:embed configs/*
var configs embed.FS

func Setup(monitoringDir string) error {
	return WriteMonitoringJSONFiles(monitoringDir)
}

func WriteMonitoringJSONFiles(monitoringDir string) error {
	dashboardDir := filepath.Join(monitoringDir, constants.DashboardsDir)
	files, err := dashboards.ReadDir(constants.DashboardsDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fileContent, err := dashboards.ReadFile(filepath.Join(constants.DashboardsDir, file.Name()))
		if err != nil {
			return err
		}
		dashboardJSONFile, err := os.Create(filepath.Join(dashboardDir, file.Name()))
		if err != nil {
			return err
		}
		_, err = dashboardJSONFile.Write(fileContent)
		if err != nil {
			return err
		}
	}
	return nil
}

func GenerateConfig(configPath string, configDesc string, templateVars configInputs) (string, error) {
	configTemplate, err := configs.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	var config bytes.Buffer
	t, err := template.New(configDesc).Parse(string(configTemplate))
	if err != nil {
		return "", err
	}
	err = t.Execute(&config, templateVars)
	if err != nil {
		return "", err
	}
	return config.String(), nil
}

func WritePrometheusConfig(filePath string, avalancheGoPorts []string, machinePorts []string) error {
	config, err := GenerateConfig("configs/prometheus.yml", "Prometheus Config", configInputs{
		AvalancheGoPorts: strings.Join(utils.AddSingleQuotes(avalancheGoPorts), ","),
		MachinePorts:     strings.Join(utils.AddSingleQuotes(machinePorts), ","),
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(config), constants.WriteReadReadPerms)
}

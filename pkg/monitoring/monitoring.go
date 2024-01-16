// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package monitoring

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

//go:embed dashboards/*
var dashboards embed.FS

//go:embed monitoring-separate-installer.sh
var monitoringScript []byte

func Setup(monitoringDir string) error {
	err := WriteMonitoringScript(monitoringDir)
	if err != nil {
		return err
	}
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

func WriteMonitoringScript(monitoringDir string) error {
	monitoringScriptFile, err := os.Create(filepath.Join(monitoringDir, constants.MonitoringScriptFile))
	if err != nil {
		return err
	}
	_, err = monitoringScriptFile.Write(monitoringScript)
	return err
}

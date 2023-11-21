// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package monitoring

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed dashboards/*
var dashboards embed.FS

func WriteMonitoringJSONFiles(monitoringDir string) error {
	dashboardDir := filepath.Join(monitoringDir, "dashboards")
	files, err := dashboards.ReadDir("dashboards")
	if err != nil {
		return err
	}

	for _, file := range files {
		fileContent, err := dashboards.ReadFile(fmt.Sprintf("%s/%s", "dashboards", file.Name()))
		if err != nil {
			return err
		}
		playbookFile, err := os.Create(filepath.Join(dashboardDir, file.Name()))
		if err != nil {
			return err
		}
		_, err = playbookFile.Write(fileContent)
		if err != nil {
			return err
		}
	}
	return nil
}

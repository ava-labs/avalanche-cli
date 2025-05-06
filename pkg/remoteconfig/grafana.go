// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import "github.com/ava-labs/avalanche-cli/pkg/utils"

func RenderGrafanaLokiDataSourceConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana-loki-datasource.yaml")
}

func RenderGrafanaPrometheusDataSourceConfigg() ([]byte, error) {
	return templates.ReadFile("templates/grafana-prometheus-datasource.yaml")
}

func RenderGrafanaConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana.ini")
}

func RenderGrafanaDashboardConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana-dashboards.yaml")
}

func GrafanaFoldersToCreate() []string {
	return []string{
		utils.GetRemoteComposeServicePath("grafana", "data"),
		utils.GetRemoteComposeServicePath("grafana", "dashboards"),
		utils.GetRemoteComposeServicePath("grafana", "provisioning", "datasources"),
		utils.GetRemoteComposeServicePath("grafana", "provisioning", "dashboards"),
	}
}

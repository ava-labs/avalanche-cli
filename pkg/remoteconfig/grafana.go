// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import "github.com/ava-labs/avalanche-cli/pkg/utils"

func RenderGrafanaLokiDataSourceConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana-loki-datasource.yaml")
}

func RenderGrafanaConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana.ini")
}

func GrafanaFoldersToCreate() []string {
	return []string{
		utils.GetRemoteComposeServicePath("grafana", "data"),
		utils.GetRemoteComposeServicePath("grafana", "provisioning", "datasources"),
		utils.GetRemoteComposeServicePath("grafana", "provisioning", "dashboards"),
	}
}

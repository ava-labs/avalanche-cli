// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

func RenderGrafanaLokiDataSourceConfig() ([]byte, error) {
	return templates.ReadFile("templates/grafana-loki-datasource.yaml")
}

func GrafanaLokiFoldersToCreate() []string {
	return []string{"/var/lib/loki",
		"/etc/grafana/provisioning/datasources",
		"/etc/grafana/provisioning/dashboards"}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/remoteconfig"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

type AvalancheGoConfigOptions struct {
	BootstrapIPs      []string
	BootstrapIDs      []string
	GenesisPath       string
	UpgradePath       string
	AllowPublicAccess bool
}

func prepareAvalanchegoConfig(
	host *models.Host,
	network models.Network,
	avalancheGoConfig AvalancheGoConfigOptions,
) (string, string, error) {
	avagoConf := remoteconfig.PrepareAvalancheConfig(host.IP, network.NetworkIDFlagValue(), nil)
	if avalancheGoConfig.AllowPublicAccess || utils.IsE2E() {
		avagoConf.HTTPHost = "0.0.0.0"
	}
	avagoConf.BootstrapIPs = strings.Join(avalancheGoConfig.BootstrapIPs, ",")
	avagoConf.BootstrapIDs = strings.Join(avalancheGoConfig.BootstrapIDs, ",")
	if avalancheGoConfig.GenesisPath != "" {
		avagoConf.GenesisPath = filepath.Join(constants.DockerNodeConfigPath, constants.GenesisFileName)
	}
	if avalancheGoConfig.UpgradePath != "" {
		avagoConf.UpgradePath = filepath.Join(constants.DockerNodeConfigPath, constants.UpgradeFileName)
	}
	nodeConf, err := remoteconfig.RenderAvalancheNodeConfig(avagoConf)
	if err != nil {
		return "", "", err
	}
	nodeConfFile, err := os.CreateTemp("", "avalanchecli-node-*.yml")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(nodeConfFile.Name(), nodeConf, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", err
	}
	cChainConf, err := remoteconfig.RenderAvalancheCChainConfig(avagoConf)
	if err != nil {
		return "", "", err
	}
	cChainConfFile, err := os.CreateTemp("", "avalanchecli-cchain-*.yml")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(cChainConfFile.Name(), cChainConf, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", err
	}
	return nodeConfFile.Name(), cChainConfFile.Name(), nil
}

func prepareGrafanaConfig() (string, string, string, string, error) {
	grafanaDataSource, err := remoteconfig.RenderGrafanaLokiDataSourceConfig()
	if err != nil {
		return "", "", "", "", err
	}
	grafanaDataSourceFile, err := os.CreateTemp("", "avalanchecli-grafana-datasource-*.yml")
	if err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(grafanaDataSourceFile.Name(), grafanaDataSource, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", "", "", err
	}

	grafanaPromDataSource, err := remoteconfig.RenderGrafanaPrometheusDataSourceConfigg()
	if err != nil {
		return "", "", "", "", err
	}
	grafanaPromDataSourceFile, err := os.CreateTemp("", "avalanchecli-grafana-prom-datasource-*.yml")
	if err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(grafanaPromDataSourceFile.Name(), grafanaPromDataSource, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", "", "", err
	}

	grafanaDashboards, err := remoteconfig.RenderGrafanaDashboardConfig()
	if err != nil {
		return "", "", "", "", err
	}
	grafanaDashboardsFile, err := os.CreateTemp("", "avalanchecli-grafana-dashboards-*.yml")
	if err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(grafanaDashboardsFile.Name(), grafanaDashboards, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", "", "", err
	}

	grafanaConfig, err := remoteconfig.RenderGrafanaConfig()
	if err != nil {
		return "", "", "", "", err
	}
	grafanaConfigFile, err := os.CreateTemp("", "avalanchecli-grafana-config-*.ini")
	if err != nil {
		return "", "", "", "", err
	}
	if err := os.WriteFile(grafanaConfigFile.Name(), grafanaConfig, constants.WriteReadUserOnlyPerms); err != nil {
		return "", "", "", "", err
	}
	return grafanaConfigFile.Name(), grafanaDashboardsFile.Name(), grafanaDataSourceFile.Name(), grafanaPromDataSourceFile.Name(), nil
}

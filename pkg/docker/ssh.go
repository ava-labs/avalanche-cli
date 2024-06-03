// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/remoteconfig"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

// ValidateComposeFile validates a docker-compose file on a remote host.
func ValidateComposeFile(host *models.Host, composeFile string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s config", composeFile), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// ComposeSSHSetupNode sets up an AvalancheGo node and dependencies on a remote host over SSH.
func ComposeSSHSetupNode(host *models.Host, network models.Network, avalancheGoVersion string, withMonitoring bool) error {
	startTime := time.Now()
	folderStructure := remoteconfig.RemoteFoldersToCreateAvalanchego()
	for _, dir := range folderStructure {
		if err := host.MkdirAll(dir, constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	ux.Logger.Info("avalancheCLI folder structure created on remote host %s after %s", folderStructure, time.Since(startTime))
	// configs
	networkID := network.NetworkIDFlagValue()
	if network.Kind == models.Local || network.Kind == models.Devnet {
		networkID = fmt.Sprintf("%d", network.ID)
	}

	avagoDockerImage := fmt.Sprintf("%s:%s", constants.AvalancheGoDockerImage, avalancheGoVersion)
	ux.Logger.Info("Preparing AvalancheGo Docker image %s on %s[%s]", avagoDockerImage, host.NodeID, host.IP)
	if err := PrepareDockerImageWithRepo(host, avagoDockerImage, constants.AvalancheGoGitRepo, avalancheGoVersion); err != nil {
		return err
	}
	ux.Logger.Info("AvalancheGo Docker image %s ready on %s[%s] after %s", avagoDockerImage, host.NodeID, host.IP, time.Since(startTime))
	nodeConfFile, cChainConfFile, err := prepareAvalanchegoConfig(host, networkID)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(nodeConfFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", nodeConfFile, err)
		}
		if err := os.Remove(cChainConfFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", cChainConfFile, err)
		}
	}()

	if err := host.Upload(nodeConfFile, remoteconfig.GetRemoteAvalancheNodeConfig(), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	if err := host.Upload(cChainConfFile, remoteconfig.GetRemoteAvalancheCChainConfig(), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	ux.Logger.Info("AvalancheGo configs uploaded to %s[%s] after %s", host.NodeID, host.IP, time.Since(startTime))
	return ComposeOverSSH("Compose Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		dockerComposeInputs{
			AvalanchegoVersion: avalancheGoVersion,
			WithMonitoring:     withMonitoring,
			WithAvalanchego:    true,
			E2E:                utils.IsE2E(),
			E2EIP:              utils.E2EConvertIP(host.IP),
			E2ESuffix:          utils.E2ESuffix(host.IP),
		})
}

func ComposeSSHSetupLoadTest(host *models.Host) error {
	return ComposeOverSSH("Compose Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		dockerComposeInputs{
			WithMonitoring:  true,
			WithAvalanchego: false,
		})
}

// WasNodeSetupWithMonitoring checks if an AvalancheGo node was setup with monitoring on a remote host.
func WasNodeSetupWithMonitoring(host *models.Host) (bool, error) {
	return HasRemoteComposeService(host, utils.GetRemoteComposeFile(), "promtail", constants.SSHScriptTimeout)
}

// ComposeSSHSetupMonitoring sets up monitoring using docker-compose.
func ComposeSSHSetupMonitoring(host *models.Host) error {
	grafanaConfigFile, grafanaDashboardsFile, grafanaLokiDatasourceFile, grafanaPromDatasourceFile, err := prepareGrafanaConfig()
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(grafanaLokiDatasourceFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", grafanaLokiDatasourceFile, err)
		}
		if err := os.Remove(grafanaPromDatasourceFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", grafanaPromDatasourceFile, err)
		}
		if err := os.Remove(grafanaDashboardsFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", grafanaDashboardsFile, err)
		}
		if err := os.Remove(grafanaConfigFile); err != nil {
			ux.Logger.Error("Error removing temporary file %s: %s", grafanaConfigFile, err)
		}
	}()

	grafanaLokiDatasourceRemoteFileName := filepath.Join(utils.GetRemoteComposeServicePath("grafana", "provisioning", "datasources"), "loki.yml")
	if err := host.Upload(grafanaLokiDatasourceFile, grafanaLokiDatasourceRemoteFileName, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	grafanaPromDatasourceFileName := filepath.Join(utils.GetRemoteComposeServicePath("grafana", "provisioning", "datasources"), "prometheus.yml")
	if err := host.Upload(grafanaPromDatasourceFile, grafanaPromDatasourceFileName, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	grafanaDashboardsRemoteFileName := filepath.Join(utils.GetRemoteComposeServicePath("grafana", "provisioning", "dashboards"), "dashboards.yml")
	if err := host.Upload(grafanaDashboardsFile, grafanaDashboardsRemoteFileName, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	grafanaConfigRemoteFileName := filepath.Join(utils.GetRemoteComposeServicePath("grafana"), "grafana.ini")
	if err := host.Upload(grafanaConfigFile, grafanaConfigRemoteFileName, constants.SSHFileOpsTimeout); err != nil {
		return err
	}

	return ComposeOverSSH("Setup Monitoring",
		host,
		constants.SSHScriptTimeout,
		"templates/monitoring.docker-compose.yml",
		dockerComposeInputs{})
}

func ComposeSSHSetupAWMRelayer(host *models.Host) error {
	return ComposeOverSSH("Setup AWM Relayer",
		host,
		constants.SSHScriptTimeout,
		"templates/awmrelayer.docker-compose.yml",
		dockerComposeInputs{})
}

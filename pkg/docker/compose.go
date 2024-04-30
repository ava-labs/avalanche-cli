// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/remoteconfig"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

type dockerComposeInputs struct {
	WithMonitoring     bool
	WithAvalanchego    bool
	AvalanchegoVersion string
}

//go:embed templates/*.docker-compose.yml
var composeTemplate embed.FS

func renderComposeFile(composePath string, composeDesc string, templateVars dockerComposeInputs) ([]byte, error) {
	compose, err := composeTemplate.ReadFile(composePath)
	if err != nil {
		return nil, err
	}
	var composeBytes bytes.Buffer
	t, err := template.New(composeDesc).Parse(string(compose))
	if err != nil {
		return nil, err
	}
	if err := t.Execute(&composeBytes, templateVars); err != nil {
		return nil, err
	}
	return composeBytes.Bytes(), nil
}

func pushComposeFile(host *models.Host, localFile string, remoteFile string, merge bool) error {
	if !utils.FileExists(localFile) {
		return fmt.Errorf("file %s does not exist", localFile)
	}
	if err := host.MkdirAll(filepath.Dir(remoteFile), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	fileExists, err := host.FileExists(remoteFile)
	if err != nil {
		return err
	}
	ux.Logger.Info("Pushing compose file %s to %s", localFile, remoteFile)
	if fileExists && merge {
		// upload new and merge files
		ux.Logger.Info("Merging compose files")
		tmpFile, err := host.CreateTemp()
		if err != nil {
			return err
		}
		defer func() {
			if err := host.Remove(tmpFile, false); err != nil {
				ux.Logger.Error("Error removing temporary file %s: %s", tmpFile, err)
			}
		}()
		if err := host.Upload(localFile, tmpFile, constants.SSHFileOpsTimeout); err != nil {
			return err
		}
		if err := mergeComposeFiles(host, remoteFile, tmpFile); err != nil {
			return err
		}
	} else {
		ux.Logger.Info("Uploading compose file")
		if err := host.Upload(localFile, remoteFile, constants.SSHFileOpsTimeout); err != nil {
			return err
		}
	}
	return nil
}

// mergeComposeFiles merges two docker-compose files on a remote host.
func mergeComposeFiles(host *models.Host, currentComposeFile string, newComposeFile string) error {
	fileExists, err := host.FileExists(currentComposeFile)
	if err != nil {
		return err
	}
	if !fileExists {
		return fmt.Errorf("file %s does not exist", currentComposeFile)
	}

	fileExists, err = host.FileExists(newComposeFile)
	if err != nil {
		return err
	}
	if !fileExists {
		return fmt.Errorf("file %s does not exist", newComposeFile)
	}

	output, err := host.Command(fmt.Sprintf("docker compose -f %s -f %s config", currentComposeFile, newComposeFile), nil, constants.SSHScriptTimeout)
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	tmpFile, err := os.CreateTemp("", "avalancecli-docker-compose-*.yml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(output); err != nil {
		return err
	}
	ux.Logger.Info("Merged compose files as %s", output)
	if err := pushComposeFile(host, tmpFile.Name(), currentComposeFile, false); err != nil {
		return err
	}
	return nil
}

func StartDockerCompose(host *models.Host, timeout time.Duration) error {
	if output, err := host.Command("sudo systemctl start avalanche-cli-docker", nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func StopDockerCompose(host *models.Host, timeout time.Duration) error {
	if output, err := host.Command("sudo systemctl stop avalanche-cli-docker", nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func RestartDockerCompose(host *models.Host, timeout time.Duration) error {
	if output, err := host.Command("sudo systemctl restart avalanche-cli-docker", nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func StartDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if err := InitDockerComposeService(host, composeFile, service, timeout); err != nil {
		return err
	}
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s start %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func StopDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s stop %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func RestartDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s restart %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func InitDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s create %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// ComposeOverSSH sets up a docker-compose file on a remote host over SSH.
func ComposeOverSSH(
	composeDesc string,
	host *models.Host,
	timeout time.Duration,
	composePath string,
	composeVars dockerComposeInputs,
) error {
	remoteComposeFile := utils.GetRemoteComposeFile()
	startTime := time.Now()
	tmpFile, err := os.CreateTemp("", "avalanchecli-docker-compose-*.yml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	composeData, err := renderComposeFile(composePath, composeDesc, composeVars)
	if err != nil {
		return err
	}

	if _, err := tmpFile.Write(composeData); err != nil {
		return err
	}
	ux.Logger.Info("pushComposeFile [%s]%s", host.NodeID, composeDesc)
	if err := pushComposeFile(host, tmpFile.Name(), remoteComposeFile, true); err != nil {
		return err
	}
	ux.Logger.Info("StartDockerCompose [%s]%s", host.NodeID, composeDesc)
	if err := StartDockerCompose(host, timeout); err != nil {
		return err
	}
	executionTime := time.Since(startTime)
	ux.Logger.Info("ComposeOverSSH[%s]%s took %s with err: %v", host.NodeID, composeDesc, executionTime, err)
	return nil
}

// ListRemoteComposeServices lists the services in a remote docker-compose file.
func ListRemoteComposeServices(host *models.Host, composeFile string, timeout time.Duration) ([]string, error) {
	output, err := host.Command(fmt.Sprintf("docker compose -f %s config --services", composeFile), nil, timeout)
	if err != nil {
		return nil, err
	}
	return utils.CleanupStrings(utils.SplitSeparatedBytesToString(output, "\n")), nil
}

// GetRemoteComposeContent gets the content of a remote docker-compose file.
func GetRemoteComposeContent(host *models.Host, composeFile string, timeout time.Duration) (string, error) {
	tmpFile, err := os.CreateTemp("", "avalancecli-docker-compose-*.yml")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if err := host.Download(composeFile, tmpFile.Name(), timeout); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseRemoteComposeContent extracts a value from a remote docker-compose file.
func ParseRemoteComposeContent(host *models.Host, composeFile string, pattern string, timeout time.Duration) (string, error) {
	content, err := GetRemoteComposeContent(host, composeFile, timeout)
	if err != nil {
		return "", err
	}
	return utils.ExtractPlaceholderValue(pattern, content)
}

// HasRemoteComposeService checks if a service is present in a remote docker-compose file.
func HasRemoteComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) (bool, error) {
	services, err := ListRemoteComposeServices(host, composeFile, timeout)
	if err != nil {
		return false, err
	}
	found := false
	for _, s := range services {
		if s == service {
			found = true
			break
		}
	}
	return found, nil
}

// ValidateComposeFile validates a docker-compose file on a remote host.
func ValidateComposeFile(host *models.Host, composeFile string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker compose -f %s config", composeFile), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func prepareAvalanchegoConfig(host *models.Host, networkID string) (string, string, error) {
	avagoConf := remoteconfig.DefaultCliAvalancheConfig(host.IP, networkID)
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

// ComposeSSHSetupNode sets up an AvalancheGo node and dependencies on a remote host over SSH.
func ComposeSSHSetupNode(host *models.Host, network models.Network, avalancheGoVersion string, withMonitoring bool) error {
	for _, dir := range remoteconfig.RemoteFoldersToCreateAvalanchego() {
		if err := host.MkdirAll(dir, constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	ux.Logger.Info("avalancheCLI folder structure created on remote host %s", remoteconfig.RemoteFoldersToCreateAvalanchego())
	// configs
	networkID := network.NetworkIDFlagValue()
	if network.Kind == models.Local || network.Kind == models.Devnet {
		networkID = fmt.Sprintf("%d", network.ID)
	}

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
	return ComposeOverSSH("Compose Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		dockerComposeInputs{
			AvalanchegoVersion: avalancheGoVersion,
			WithMonitoring:     withMonitoring,
			WithAvalanchego:    true,
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

// WasNodeSetupWithTeleporter checks if an AvalancheGo node was setup with teleporter on a remote host.
func WasNodeSetupWithTeleporter(host *models.Host) (bool, error) {
	return HasRemoteComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHScriptTimeout)
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

// ComposeSSHSetupCChain sets up an Avalanche C-Chain node and dependencies on a remote host over SSH.
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
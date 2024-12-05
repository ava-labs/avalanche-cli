// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/docker"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/monitoring"
	"github.com/ava-labs/avalanche-cli/pkg/remoteconfig"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
)

type scriptInputs struct {
	AvalancheGoVersion      string
	SubnetExportFileName    string
	SubnetName              string
	ClusterName             string
	GoVersion               string
	IsDevNet                bool
	IsE2E                   bool
	NetworkFlag             string
	VMBinaryPath            string
	SubnetEVMReleaseURL     string
	SubnetEVMArchive        string
	MonitoringDashboardPath string
	LoadTestRepoDir         string
	LoadTestRepo            string
	LoadTestPath            string
	LoadTestCommand         string
	LoadTestBranch          string
	LoadTestGitCommit       string
	CheckoutCommit          bool
	LoadTestResultFile      string
	GrafanaPkg              string
	CustomVMRepoDir         string
	CustomVMRepoURL         string
	CustomVMBranch          string
	CustomVMBuildScript     string
}

//go:embed shell/*.sh
var script embed.FS

// RunOverSSH runs provided script path over ssh.
// This script can be template as it will be rendered using scriptInputs vars
func RunOverSSH(
	scriptDesc string,
	host *models.Host,
	timeout time.Duration,
	scriptPath string,
	templateVars scriptInputs,
) error {
	startTime := time.Now()
	shellScript, err := script.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	var script bytes.Buffer
	t, err := template.New(scriptDesc).Parse(string(shellScript))
	if err != nil {
		return err
	}
	err = t.Execute(&script, templateVars)
	if err != nil {
		return err
	}

	if output, err := host.Command(script.String(), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	executionTime := time.Since(startTime)
	ux.Logger.Info("RunOverSSH[%s]%s took %s with err: %v", host.NodeID, scriptDesc, executionTime, err)
	return nil
}

func PostOverSSH(host *models.Host, path string, requestBody string) ([]byte, error) {
	if path == "" {
		path = "/ext/info"
	}
	localhost, err := url.Parse(constants.LocalAPIEndpoint)
	if err != nil {
		return nil, err
	}
	requestHeaders := fmt.Sprintf("POST %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Content-Length: %d\r\n"+
		"Content-Type: application/json\r\n\r\n", path, localhost.Host, len(requestBody))
	httpRequest := requestHeaders + requestBody
	return host.Forward(httpRequest, constants.SSHPOSTTimeout)
}

// RunSSHSetupNode runs script to setup node
func RunSSHSetupNode(host *models.Host, configPath string) error {
	if err := RunOverSSH(
		"Setup Node",
		host,
		constants.SSHLongRunningScriptTimeout,
		"shell/setupNode.sh",
		scriptInputs{IsE2E: utils.IsE2E()},
	); err != nil {
		return err
	}
	// name: copy metrics config to cloud server
	ux.Logger.Info("Uploading config %s to server %s: %s", configPath, host.NodeID, filepath.Join(constants.CloudNodeCLIConfigBasePath, filepath.Base(configPath)))
	if err := host.Upload(
		configPath,
		filepath.Join(constants.CloudNodeCLIConfigBasePath, filepath.Base(configPath)),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	return nil
}

// RunSSHSetupDockerService runs script to setup docker compose service for CLI
func RunSSHSetupDockerService(host *models.Host) error {
	if host.IsSystemD() {
		return RunOverSSH(
			"Setup Docker Service",
			host,
			constants.SSHLongRunningScriptTimeout,
			"shell/setupDockerService.sh",
			scriptInputs{},
		)
	} else {
		// no need to setup docker service
		return nil
	}
}

// RunSSHRestartNode runs script to restart avalanchego
func RunSSHRestartNode(host *models.Host) error {
	remoteComposeFile := utils.GetRemoteComposeFile()
	avagoService := "avalanchego"
	if utils.IsE2E() {
		avagoService += utils.E2ESuffix(host.IP)
	}
	return docker.RestartDockerComposeService(host, remoteComposeFile, avagoService, constants.SSHLongRunningScriptTimeout)
}

// ComposeSSHSetupICMRelayer used docker compose to setup AWM Relayer
func ComposeSSHSetupICMRelayer(host *models.Host, relayerVersion string) error {
	if err := docker.ComposeSSHSetupICMRelayer(host, relayerVersion); err != nil {
		return err
	}
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHStartICMRelayerService runs script to start an AWM Relayer Service
func RunSSHStartICMRelayerService(host *models.Host) error {
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHStopICMRelayerService runs script to start an AWM Relayer Service
func RunSSHStopICMRelayerService(host *models.Host) error {
	return docker.StopDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHUpgradeAvalanchego runs script to upgrade avalanchego
func RunSSHUpgradeAvalanchego(host *models.Host, avalancheGoVersion string) error {
	withMonitoring, err := docker.WasNodeSetupWithMonitoring(host)
	if err != nil {
		return err
	}
	if err := docker.ComposeOverSSH("Compose Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		docker.DockerComposeInputs{
			AvalanchegoVersion: avalancheGoVersion,
			WithMonitoring:     withMonitoring,
			WithAvalanchego:    true,
			E2E:                utils.IsE2E(),
			E2EIP:              utils.E2EConvertIP(host.IP),
			E2ESuffix:          utils.E2ESuffix(host.IP),
		}); err != nil {
		return err
	}
	return docker.RestartDockerCompose(host, constants.SSHLongRunningScriptTimeout)
}

// RunSSHStartNode runs script to start avalanchego
func RunSSHStartNode(host *models.Host) error {
	if utils.IsE2E() && utils.E2EDocker() {
		return RunOverSSH(
			"E2E Start Avalanchego",
			host,
			constants.SSHScriptTimeout,
			"shell/e2e_startNode.sh",
			scriptInputs{},
		)
	}
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout)
}

// RunSSHStopNode runs script to stop avalanchego
func RunSSHStopNode(host *models.Host) error {
	if utils.IsE2E() && utils.E2EDocker() {
		return RunOverSSH(
			"E2E Stop Avalanchego",
			host,
			constants.SSHScriptTimeout,
			"shell/e2e_stopNode.sh",
			scriptInputs{},
		)
	}
	return docker.StopDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout)
}

func replaceCustomVarDashboardValues(customGrafanaDashboardFileName, chainID string) error {
	content, err := os.ReadFile(customGrafanaDashboardFileName)
	if err != nil {
		return err
	}
	replacements := []struct {
		old string
		new string
	}{
		{"\"text\": \"CHAIN_ID_VAL\"", fmt.Sprintf("\"text\": \"%v\"", chainID)},
		{"\"value\": \"CHAIN_ID_VAL\"", fmt.Sprintf("\"value\": \"%v\"", chainID)},
		{"\"query\": \"CHAIN_ID_VAL\"", fmt.Sprintf("\"query\": \"%v\"", chainID)},
	}
	for _, r := range replacements {
		content = []byte(strings.ReplaceAll(string(content), r.old, r.new))
	}
	err = os.WriteFile(customGrafanaDashboardFileName, content, constants.WriteReadUserOnlyPerms)
	if err != nil {
		return err
	}
	return nil
}

func RunSSHUpdateMonitoringDashboards(host *models.Host, monitoringDashboardPath, customGrafanaDashboardPath, chainID string) error {
	remoteDashboardsPath := utils.GetRemoteComposeServicePath("grafana", "dashboards")
	if !sdkutils.DirExists(monitoringDashboardPath) {
		return fmt.Errorf("%s does not exist", monitoringDashboardPath)
	}
	if customGrafanaDashboardPath != "" && utils.FileExists(utils.ExpandHome(customGrafanaDashboardPath)) {
		if err := utils.FileCopy(utils.ExpandHome(customGrafanaDashboardPath), filepath.Join(monitoringDashboardPath, constants.CustomGrafanaDashboardJSON)); err != nil {
			return err
		}
		if err := replaceCustomVarDashboardValues(filepath.Join(monitoringDashboardPath, constants.CustomGrafanaDashboardJSON), chainID); err != nil {
			return err
		}
	}
	if err := host.MkdirAll(remoteDashboardsPath, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(monitoringDashboardPath, constants.CustomGrafanaDashboardJSON),
		filepath.Join(remoteDashboardsPath, constants.CustomGrafanaDashboardJSON),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	return docker.RestartDockerComposeService(host, utils.GetRemoteComposeFile(), "grafana", constants.SSHLongRunningScriptTimeout)
}

func RunSSHSetupMonitoringFolders(host *models.Host) error {
	for _, folder := range remoteconfig.RemoteFoldersToCreateMonitoring() {
		if err := host.MkdirAll(folder, constants.SSHDirOpsTimeout); err != nil {
			return err
		}
	}
	return nil
}

func RunSSHCopyMonitoringDashboards(host *models.Host, monitoringDashboardPath string) error {
	// TODO: download dashboards from github instead
	remoteDashboardsPath := utils.GetRemoteComposeServicePath("grafana", "dashboards")
	if !sdkutils.DirExists(monitoringDashboardPath) {
		return fmt.Errorf("%s does not exist", monitoringDashboardPath)
	}
	if err := host.MkdirAll(remoteDashboardsPath, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	dashboards, err := os.ReadDir(monitoringDashboardPath)
	if err != nil {
		return err
	}
	for _, dashboard := range dashboards {
		if err := host.Upload(
			filepath.Join(monitoringDashboardPath, dashboard.Name()),
			filepath.Join(remoteDashboardsPath, dashboard.Name()),
			constants.SSHFileOpsTimeout,
		); err != nil {
			return err
		}
	}
	if composeFileExists(host) {
		return docker.RestartDockerComposeService(host, utils.GetRemoteComposeFile(), "grafana", constants.SSHLongRunningScriptTimeout)
	} else {
		return nil
	}
}

func RunSSHCopyYAMLFile(host *models.Host, yamlFilePath string) error {
	if err := host.Upload(
		yamlFilePath,
		fmt.Sprintf("/home/ubuntu/%s", filepath.Base(yamlFilePath)),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	return nil
}

func RunSSHSetupPrometheusConfig(host *models.Host, avalancheGoPorts, machinePorts, loadTestPorts []string) error {
	for _, folder := range remoteconfig.PrometheusFoldersToCreate() {
		if err := host.MkdirAll(folder, constants.SSHDirOpsTimeout); err != nil {
			return err
		}
	}
	cloudNodePrometheusConfigTemp := utils.GetRemoteComposeServicePath("prometheus", "prometheus.yml")
	promConfig, err := os.CreateTemp("", "prometheus")
	if err != nil {
		return err
	}
	defer os.Remove(promConfig.Name())
	if err := monitoring.WritePrometheusConfig(promConfig.Name(), avalancheGoPorts, machinePorts, loadTestPorts); err != nil {
		return err
	}

	return host.Upload(
		promConfig.Name(),
		cloudNodePrometheusConfigTemp,
		constants.SSHFileOpsTimeout,
	)
}

func RunSSHSetupLokiConfig(host *models.Host, port int) error {
	for _, folder := range remoteconfig.LokiFoldersToCreate() {
		if err := host.MkdirAll(folder, constants.SSHDirOpsTimeout); err != nil {
			return err
		}
	}
	cloudNodeLokiConfigTemp := utils.GetRemoteComposeServicePath("loki", "loki.yml")
	lokiConfig, err := os.CreateTemp("", "loki")
	if err != nil {
		return err
	}
	defer os.Remove(lokiConfig.Name())
	if err := monitoring.WriteLokiConfig(lokiConfig.Name(), strconv.Itoa(port)); err != nil {
		return err
	}
	return host.Upload(
		lokiConfig.Name(),
		cloudNodeLokiConfigTemp,
		constants.SSHFileOpsTimeout,
	)
}

func RunSSHSetupPromtailConfig(host *models.Host, lokiIP string, lokiPort int, cloudID string, nodeID string, chainID string) error {
	for _, folder := range remoteconfig.PromtailFoldersToCreate() {
		if err := host.MkdirAll(folder, constants.SSHDirOpsTimeout); err != nil {
			return err
		}
	}
	cloudNodePromtailConfigTemp := utils.GetRemoteComposeServicePath("promtail", "promtail.yml")
	promtailConfig, err := os.CreateTemp("", "promtail")
	if err != nil {
		return err
	}
	defer os.Remove(promtailConfig.Name())

	if err := monitoring.WritePromtailConfig(promtailConfig.Name(), lokiIP, strconv.Itoa(lokiPort), cloudID, nodeID, chainID); err != nil {
		return err
	}
	return host.Upload(
		promtailConfig.Name(),
		cloudNodePromtailConfigTemp,
		constants.SSHFileOpsTimeout,
	)
}

func RunSSHDownloadNodePrometheusConfig(host *models.Host, nodeInstanceDirPath string) error {
	return host.Download(
		constants.CloudNodePrometheusConfigPath,
		filepath.Join(nodeInstanceDirPath, constants.NodePrometheusConfigFileName),
		constants.SSHFileOpsTimeout,
	)
}

func RunSSHUploadNodeICMRelayerConfig(host *models.Host, nodeInstanceDirPath string) error {
	cloudICMRelayerConfigDir := filepath.Join(constants.CloudNodeCLIConfigBasePath, constants.ServicesDir, constants.ICMRelayerInstallDir)
	if err := host.MkdirAll(cloudICMRelayerConfigDir, constants.SSHDirOpsTimeout); err != nil {
		return err
	}
	return host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.ServicesDir, constants.ICMRelayerInstallDir, constants.ICMRelayerConfigFilename),
		filepath.Join(cloudICMRelayerConfigDir, constants.ICMRelayerConfigFilename),
		constants.SSHFileOpsTimeout,
	)
}

// RunSSHGetNewSubnetEVMRelease runs script to download new subnet evm
func RunSSHGetNewSubnetEVMRelease(host *models.Host, subnetEVMReleaseURL, subnetEVMArchive string) error {
	return RunOverSSH(
		"Get Subnet EVM Release",
		host,
		constants.SSHScriptTimeout,
		"shell/getNewSubnetEVMRelease.sh",
		scriptInputs{SubnetEVMReleaseURL: subnetEVMReleaseURL, SubnetEVMArchive: subnetEVMArchive},
	)
}

// RunSSHSetupDevNet runs script to setup devnet
func RunSSHSetupDevNet(host *models.Host, nodeInstanceDirPath string) error {
	if err := host.MkdirAll(
		constants.CloudNodeConfigPath,
		constants.SSHDirOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.GenesisFileName),
		remoteconfig.GetRemoteAvalancheGenesis(),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.UpgradeFileName),
		remoteconfig.GetRemoteAvalancheUpgrade(),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.NodeFileName),
		remoteconfig.GetRemoteAvalancheNodeConfig(),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	if err := docker.StopDockerCompose(host, constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	if err := host.Remove("/home/ubuntu/.avalanchego/db", true); err != nil {
		return err
	}
	if err := host.MkdirAll("/home/ubuntu/.avalanchego/db", constants.SSHDirOpsTimeout); err != nil {
		return err
	}
	if err := host.Remove("/home/ubuntu/.avalanchego/logs", true); err != nil {
		return err
	}
	if err := host.MkdirAll("/home/ubuntu/.avalanchego/logs", constants.SSHDirOpsTimeout); err != nil {
		return err
	}
	return docker.StartDockerCompose(host, constants.SSHLongRunningScriptTimeout)
}

// RunSSHUploadStakingFiles uploads staking files to a remote host via SSH.
func RunSSHUploadStakingFiles(host *models.Host, nodeInstanceDirPath string) error {
	if err := host.MkdirAll(
		constants.CloudNodeStakingPath,
		constants.SSHDirOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.StakerCertFileName),
		filepath.Join(constants.CloudNodeStakingPath, constants.StakerCertFileName),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.StakerKeyFileName),
		filepath.Join(constants.CloudNodeStakingPath, constants.StakerKeyFileName),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	return host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.BLSKeyFileName),
		filepath.Join(constants.CloudNodeStakingPath, constants.BLSKeyFileName),
		constants.SSHFileOpsTimeout,
	)
}

// RunSSHRenderAvagoAliasConfigFile renders avalanche alias config to a remote host via SSH.
func RunSSHRenderAvagoAliasConfigFile(
	host *models.Host,
	blockchainID string,
	subnetAliases []string,
) error {
	aliasToBlockchain := map[string]string{}
	if aliasConfigFileExists(host) {
		// load remote aliases
		remoteAliases, err := getAvalancheGoAliasData(host)
		if err != nil {
			return err
		}
		for chainID, aliases := range remoteAliases {
			for _, alias := range aliases {
				aliasToBlockchain[alias] = chainID
			}
		}
	}
	for _, alias := range subnetAliases {
		aliasToBlockchain[alias] = blockchainID
	}
	newAliases := map[string][]string{}
	for alias, chainID := range aliasToBlockchain {
		newAliases[chainID] = append(newAliases[chainID], alias)
	}
	aliasConf, err := json.MarshalIndent(newAliases, "", "  ")
	if err != nil {
		return err
	}
	aliasConfFile, err := os.CreateTemp("", "avalanchecli-alias-*.yml")
	if err != nil {
		return err
	}
	defer os.Remove(aliasConfFile.Name())
	if err := os.WriteFile(aliasConfFile.Name(), aliasConf, constants.DefaultPerms755); err != nil {
		return err
	}
	if err := host.Upload(aliasConfFile.Name(), remoteconfig.GetRemoteAvalancheAliasesConfig(), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	return nil
}

// RunSSHRenderAvalancheNodeConfig renders avalanche node config to a remote host via SSH.
func RunSSHRenderAvalancheNodeConfig(
	app *application.Avalanche,
	host *models.Host,
	network models.Network,
	trackSubnets []string,
	isAPIHost bool,
) error {
	// get subnet ids
	subnetIDs, err := utils.MapWithError(trackSubnets, func(subnetName string) (string, error) {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", err
		} else {
			return sc.Networks[network.Name()].SubnetID.String(), nil
		}
	})
	if err != nil {
		return err
	}

	avagoConf := remoteconfig.PrepareAvalancheConfig(host.IP, network.NetworkIDFlagValue(), subnetIDs)
	// preserve remote configuration if it exists
	if nodeConfigFileExists(host) {
		// make sure that genesis and bootstrap data is preserved
		if genesisFileExists(host) {
			avagoConf.GenesisPath = filepath.Join(constants.DockerNodeConfigPath, constants.GenesisFileName)
		}
		if upgradeFileExists(host) {
			avagoConf.UpgradePath = filepath.Join(constants.DockerNodeConfigPath, constants.UpgradeFileName)
		}
		if network.Kind == models.Local || network.Kind == models.Devnet || network.Kind == models.EtnaDevnet || isAPIHost {
			avagoConf.HTTPHost = "0.0.0.0"
		}
		remoteAvagoConf, err := getAvalancheGoConfigData(host)
		if err != nil {
			return err
		}
		// ignore errors if bootstrap configuration is not present - it's fine
		bootstrapIDs, _ := utils.StringValue(remoteAvagoConf, "bootstrap-ids")
		bootstrapIPs, _ := utils.StringValue(remoteAvagoConf, "bootstrap-ips")
		avagoConf.BootstrapIDs = bootstrapIDs
		avagoConf.BootstrapIPs = bootstrapIPs
		partialSyncI, ok := remoteAvagoConf[config.PartialSyncPrimaryNetworkKey]
		if ok {
			partialSync, ok := partialSyncI.(bool)
			if ok {
				avagoConf.PartialSync = partialSync
			}
		}
	}
	// ready to render node config
	nodeConf, err := remoteconfig.RenderAvalancheNodeConfig(avagoConf)
	if err != nil {
		return err
	}
	return host.UploadBytes(nodeConf, remoteconfig.GetRemoteAvalancheNodeConfig(), constants.SSHFileOpsTimeout)
}

// RunSSHCreatePlugin runs script to create plugin
func RunSSHCreatePlugin(host *models.Host, sc models.Sidecar) error {
	vmID, err := sc.GetVMID()
	if err != nil {
		return err
	}
	subnetVMBinaryPath := fmt.Sprintf(constants.CloudNodeSubnetEvmBinaryPath, vmID)
	hostInstaller := NewHostInstaller(host)
	tmpDir, err := host.CreateTempDir()
	if err != nil {
		return err
	}
	defer func(h *models.Host) {
		_ = h.Remove(tmpDir, true)
	}(host)
	switch {
	case sc.VM == models.CustomVM:
		ux.Logger.Info("Building Custom VM for %s to %s", host.NodeID, subnetVMBinaryPath)
		ux.Logger.Info("Custom VM Params: repo %s branch %s via %s", sc.CustomVMRepoURL, sc.CustomVMBranch, sc.CustomVMBuildScript)
		if err := RunOverSSH(
			"Build CustomVM",
			host,
			constants.SSHLongRunningScriptTimeout,
			"shell/buildCustomVM.sh",
			scriptInputs{
				CustomVMRepoDir:     tmpDir,
				CustomVMRepoURL:     sc.CustomVMRepoURL,
				CustomVMBranch:      sc.CustomVMBranch,
				CustomVMBuildScript: sc.CustomVMBuildScript,
				VMBinaryPath:        subnetVMBinaryPath,
				GoVersion:           constants.BuildEnvGolangVersion,
			},
		); err != nil {
			return err
		}

	case sc.VM == models.SubnetEvm:
		ux.Logger.Info("Installing Subnet EVM for %s", host.NodeID)
		dl := binutils.NewSubnetEVMDownloader()
		installURL, _, err := dl.GetDownloadURL(sc.VMVersion, hostInstaller) // extension is tar.gz
		if err != nil {
			return err
		}

		archiveName := "subnet-evm.tar.gz"
		archiveFullPath := filepath.Join(tmpDir, archiveName)

		// download and install subnet evm
		if _, err := host.Command(fmt.Sprintf("%s %s -O %s", "busybox wget", installURL, archiveFullPath), nil, constants.SSHLongRunningScriptTimeout); err != nil {
			return err
		}
		if _, err := host.Command(fmt.Sprintf("tar -xzf %s -C %s", archiveFullPath, tmpDir), nil, constants.SSHLongRunningScriptTimeout); err != nil {
			return err
		}

		if _, err := host.Command(fmt.Sprintf("mv -f %s/subnet-evm %s", tmpDir, subnetVMBinaryPath), nil, constants.SSHLongRunningScriptTimeout); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected error: unsupported VM type: %s", sc.VM)
	}

	return nil
}

// RunSSHMergeSubnetNodeConfig merges subnet node config to the node config on the remote host
func mergeSubnetNodeConfig(host *models.Host, subnetNodeConfigPath string) error {
	if subnetNodeConfigPath == "" {
		return fmt.Errorf("subnet node config path is empty")
	}
	remoteNodeConfigBytes, err := host.ReadFileBytes(remoteconfig.GetRemoteAvalancheNodeConfig(), constants.SSHFileOpsTimeout)
	if err != nil {
		return fmt.Errorf("error reading remote node config: %w", err)
	}
	var remoteNodeConfig map[string]interface{}
	if err := json.Unmarshal(remoteNodeConfigBytes, &remoteNodeConfig); err != nil {
		return fmt.Errorf("error unmarshalling remote node config: %w", err)
	}
	subnetNodeConfigBytes, err := os.ReadFile(subnetNodeConfigPath)
	if err != nil {
		return fmt.Errorf("error reading subnet node config: %w", err)
	}
	var subnetNodeConfig map[string]interface{}
	if err := json.Unmarshal(subnetNodeConfigBytes, &subnetNodeConfig); err != nil {
		return fmt.Errorf("error unmarshalling subnet node config: %w", err)
	}
	maps.Copy(remoteNodeConfig, subnetNodeConfig) // merge remote config into local subnet config. subnetNodeConfig takes precedence
	mergedNodeConfigBytes, err := json.MarshalIndent(remoteNodeConfig, "", " ")
	if err != nil {
		return fmt.Errorf("error creating merged node config: %w", err)
	}
	return host.UploadBytes(mergedNodeConfigBytes, remoteconfig.GetRemoteAvalancheNodeConfig(), constants.SSHFileOpsTimeout)
}

// RunSSHSyncSubnetData syncs subnet data required
func RunSSHSyncSubnetData(app *application.Avalanche, host *models.Host, network models.Network, subnetName string) error {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errors.New("subnet id is empty")
	}
	subnetIDStr := subnetID.String()
	blockchainID := sc.Networks[network.Name()].BlockchainID
	// genesis config
	genesisFilename := filepath.Join(app.GetNodesDir(), host.GetCloudID(), constants.GenesisFileName)
	if utils.FileExists(genesisFilename) {
		if err := host.Upload(genesisFilename, remoteconfig.GetRemoteAvalancheGenesis(), constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("error uploading genesis config to %s: %w", remoteconfig.GetRemoteAvalancheGenesis(), err)
		}
	}
	// end genesis config
	// subnet node config
	subnetNodeConfigPath := app.GetAvagoNodeConfigPath(subnetName)
	if utils.FileExists(subnetNodeConfigPath) {
		if err := mergeSubnetNodeConfig(host, subnetNodeConfigPath); err != nil {
			return err
		}
	}
	// subnet config
	if app.AvagoSubnetConfigExists(subnetName) {
		subnetConfig, err := app.LoadRawAvagoSubnetConfig(subnetName)
		if err != nil {
			return fmt.Errorf("error loading subnet config: %w", err)
		}
		subnetConfigPath := filepath.Join(constants.CloudNodeConfigPath, "subnets", subnetIDStr+".json")
		if err := host.MkdirAll(filepath.Dir(subnetConfigPath), constants.SSHDirOpsTimeout); err != nil {
			return err
		}
		if err := host.UploadBytes(subnetConfig, subnetConfigPath, constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("error uploading subnet config to %s: %w", subnetConfigPath, err)
		}
	}
	// end subnet config

	// chain config
	if blockchainID != ids.Empty && app.ChainConfigExists(subnetName) {
		chainConfig, err := app.LoadRawChainConfig(subnetName)
		if err != nil {
			return fmt.Errorf("error loading chain config: %w", err)
		}
		chainConfigPath := filepath.Join(constants.CloudNodeConfigPath, "chains", blockchainID.String(), "config.json")
		if err := host.MkdirAll(filepath.Dir(chainConfigPath), constants.SSHDirOpsTimeout); err != nil {
			return err
		}
		if err := host.UploadBytes(chainConfig, chainConfigPath, constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("error uploading chain config to %s: %w", chainConfigPath, err)
		}
	}
	// end chain config

	// network upgrade
	if app.NetworkUpgradeExists(subnetName) {
		networkUpgrades, err := app.LoadRawNetworkUpgrades(subnetName)
		if err != nil {
			return fmt.Errorf("error loading network upgrades: %w", err)
		}
		networkUpgradesPath := filepath.Join(constants.CloudNodeConfigPath, "subnets", "chains", blockchainID.String(), "upgrade.json")
		if err := host.MkdirAll(filepath.Dir(networkUpgradesPath), constants.SSHDirOpsTimeout); err != nil {
			return err
		}
		if err := host.UploadBytes(networkUpgrades, networkUpgradesPath, constants.SSHFileOpsTimeout); err != nil {
			return fmt.Errorf("error uploading network upgrades to %s: %w", networkUpgradesPath, err)
		}
	}
	// end network upgrade

	return nil
}

func RunSSHBuildLoadTestCode(host *models.Host, loadTestRepo, loadTestPath, loadTestGitCommit, repoDirName, loadTestBranch string, checkoutCommit bool) error {
	return StreamOverSSH(
		"Build Load Test",
		host,
		constants.SSHLongRunningScriptTimeout,
		"shell/buildLoadTest.sh",
		scriptInputs{
			LoadTestRepoDir: repoDirName,
			LoadTestRepo:    loadTestRepo, LoadTestPath: loadTestPath, LoadTestGitCommit: loadTestGitCommit,
			CheckoutCommit: checkoutCommit, LoadTestBranch: loadTestBranch,
		},
	)
}

func RunSSHBuildLoadTestDependencies(host *models.Host) error {
	return RunOverSSH(
		"Build Load Test",
		host,
		constants.SSHLongRunningScriptTimeout,
		"shell/buildLoadTestDeps.sh",
		scriptInputs{GoVersion: constants.BuildEnvGolangVersion},
	)
}

func RunSSHRunLoadTest(host *models.Host, loadTestCommand, loadTestName string) error {
	return RunOverSSH(
		"Run Load Test",
		host,
		constants.SSHLongRunningScriptTimeout,
		"shell/runLoadTest.sh",
		scriptInputs{
			GoVersion:          constants.BuildEnvGolangVersion,
			LoadTestCommand:    loadTestCommand,
			LoadTestResultFile: fmt.Sprintf("/home/ubuntu/.avalanchego/logs/loadtest_%s.txt", loadTestName),
		},
	)
}

// RunSSHCheckAvalancheGoVersion checks node avalanchego version
func RunSSHCheckAvalancheGoVersion(host *models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.getNodeVersion\"}"
	return PostOverSSH(host, "", requestBody)
}

// RunSSHCheckBootstrapped checks if node is bootstrapped to primary network
func RunSSHCheckBootstrapped(host *models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.isBootstrapped\", \"params\": {\"chain\":\"X\"}}"
	return PostOverSSH(host, "", requestBody)
}

// RunSSHCheckHealthy checks if node is healthy
func RunSSHCheckHealthy(host *models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\":\"health.health\",\"params\": {\"tags\": [\"P\"]}}"
	return PostOverSSH(host, "/ext/health", requestBody)
}

// RunSSHGetNodeID reads nodeID from avalanchego
func RunSSHGetNodeID(host *models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.getNodeID\"}"
	return PostOverSSH(host, "", requestBody)
}

// SubnetSyncStatus checks if node is synced to subnet
func RunSSHSubnetSyncStatus(host *models.Host, blockchainID string) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := fmt.Sprintf("{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"platform.getBlockchainStatus\", \"params\": {\"blockchainID\":\"%s\"}}", blockchainID)
	return PostOverSSH(host, "/ext/bc/P", requestBody)
}

// StreamOverSSH runs provided script path over ssh.
// This script can be template as it will be rendered using scriptInputs vars
func StreamOverSSH(
	scriptDesc string,
	host *models.Host,
	timeout time.Duration,
	scriptPath string,
	templateVars scriptInputs,
) error {
	shellScript, err := script.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	var script bytes.Buffer
	t, err := template.New(scriptDesc).Parse(string(shellScript))
	if err != nil {
		return err
	}
	err = t.Execute(&script, templateVars)
	if err != nil {
		return err
	}

	if err := host.StreamSSHCommand(script.String(), nil, timeout); err != nil {
		return err
	}
	return nil
}

// RunSSHWhitelistPubKey downloads the authorized_keys file from the specified host, appends the provided sshPubKey to it, and uploads the file back to the host.
func RunSSHWhitelistPubKey(host *models.Host, sshPubKey string) error {
	const sshAuthFile = "/home/ubuntu/.ssh/authorized_keys"
	tmpName := filepath.Join(os.TempDir(), utils.RandomString(10))
	defer os.Remove(tmpName)
	if err := host.Download(sshAuthFile, tmpName, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	// write ssh public key
	tmpFile, err := os.OpenFile(tmpName, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := tmpFile.WriteString(sshPubKey + "\n"); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return host.Upload(tmpFile.Name(), sshAuthFile, constants.SSHFileOpsTimeout)
}

// RunSSHDownloadFile downloads specified file from the specified host
func RunSSHDownloadFile(host *models.Host, filePath string, localFilePath string) error {
	return host.Download(filePath, localFilePath, constants.SSHFileOpsTimeout)
}

func RunSSHUpsizeRootDisk(host *models.Host) error {
	return RunOverSSH(
		"Upsize Disk",
		host,
		constants.SSHScriptTimeout,
		"shell/upsizeRootDisk.sh",
		scriptInputs{},
	)
}

// composeFileExists checks if the docker-compose file exists on the host
func composeFileExists(host *models.Host) bool {
	composeFileExists, _ := host.FileExists(utils.GetRemoteComposeFile())
	return composeFileExists
}

func genesisFileExists(host *models.Host) bool {
	genesisFileExists, _ := host.FileExists(filepath.Join(constants.CloudNodeConfigPath, constants.GenesisFileName))
	return genesisFileExists
}

func upgradeFileExists(host *models.Host) bool {
	upgradeFileExists, _ := host.FileExists(filepath.Join(constants.CloudNodeConfigPath, constants.UpgradeFileName))
	return upgradeFileExists
}

func nodeConfigFileExists(host *models.Host) bool {
	nodeConfigFileExists, _ := host.FileExists(remoteconfig.GetRemoteAvalancheNodeConfig())
	return nodeConfigFileExists
}

func aliasConfigFileExists(host *models.Host) bool {
	aliasConfigFileExists, _ := host.FileExists(remoteconfig.GetRemoteAvalancheAliasesConfig())
	return aliasConfigFileExists
}

func getAvalancheGoConfigData(host *models.Host) (map[string]interface{}, error) {
	// get remote node.json file
	nodeJSONPath := filepath.Join(constants.CloudNodeConfigPath, constants.NodeConfigJSONFile)
	// parse node.json file
	nodeJSON, err := host.ReadFileBytes(nodeJSONPath, constants.SSHFileOpsTimeout)
	if err != nil {
		return nil, err
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(nodeJSON, &avagoConfig); err != nil {
		return nil, err
	}
	return avagoConfig, nil
}

func getAvalancheGoAliasData(host *models.Host) (map[string][]string, error) {
	// parse aliases.json file
	aliasesJSON, err := host.ReadFileBytes(remoteconfig.GetRemoteAvalancheAliasesConfig(), constants.SSHFileOpsTimeout)
	if err != nil {
		return nil, err
	}
	var aliases map[string][]string
	if err := json.Unmarshal(aliasesJSON, &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

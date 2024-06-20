// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"bytes"
	"embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/docker"
	"github.com/ava-labs/avalanche-cli/pkg/monitoring"
	"github.com/ava-labs/avalanche-cli/pkg/remoteconfig"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

type scriptInputs struct {
	AvalancheGoVersion      string
	CLIVersion              string
	SubnetExportFileName    string
	SubnetName              string
	ClusterName             string
	GoVersion               string
	CliBranch               string
	IsDevNet                bool
	IsE2E                   bool
	NetworkFlag             string
	SubnetEVMBinaryPath     string
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
func RunSSHSetupNode(host *models.Host, configPath, cliVersion string) error {
	if err := RunOverSSH(
		"Setup Node",
		host,
		constants.SSHLongRunningScriptTimeout,
		"shell/setupNode.sh",
		scriptInputs{CLIVersion: cliVersion, IsE2E: utils.IsE2E()},
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

// ComposeSSHSetupAWMRelayer used docker compose to setup AWM Relayer
func ComposeSSHSetupAWMRelayer(host *models.Host) error {
	if err := docker.ComposeSSHSetupAWMRelayer(host); err != nil {
		return err
	}
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHStartAWMRelayerService runs script to start an AWM Relayer Service
func RunSSHStartAWMRelayerService(host *models.Host) error {
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHStopAWMRelayerService runs script to start an AWM Relayer Service
func RunSSHStopAWMRelayerService(host *models.Host) error {
	return docker.StopDockerComposeService(host, utils.GetRemoteComposeFile(), "awm-relayer", constants.SSHLongRunningScriptTimeout)
}

// RunSSHUpgradeAvalanchego runs script to upgrade avalanchego
func RunSSHUpgradeAvalanchego(host *models.Host, network models.Network, avalancheGoVersion string) error {
	withMonitoring, err := docker.WasNodeSetupWithMonitoring(host)
	if err != nil {
		return err
	}

	if err := docker.ComposeSSHSetupNode(host, network, avalancheGoVersion, withMonitoring); err != nil {
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

// RunSSHUpgradeSubnetEVM runs script to upgrade subnet evm
func RunSSHUpgradeSubnetEVM(host *models.Host, subnetEVMBinaryPath string) error {
	return RunOverSSH(
		"Upgrade Subnet EVM",
		host,
		constants.SSHScriptTimeout,
		"shell/upgradeSubnetEVM.sh",
		scriptInputs{SubnetEVMBinaryPath: subnetEVMBinaryPath},
	)
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
	if !utils.DirectoryExists(monitoringDashboardPath) {
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
	if !utils.DirectoryExists(monitoringDashboardPath) {
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

func RunSSHUploadNodeAWMRelayerConfig(host *models.Host, nodeInstanceDirPath string) error {
	cloudAWMRelayerConfigDir := filepath.Join(constants.CloudNodeCLIConfigBasePath, constants.ServicesDir, constants.AWMRelayerInstallDir)
	if err := host.MkdirAll(cloudAWMRelayerConfigDir, constants.SSHDirOpsTimeout); err != nil {
		return err
	}
	return host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.ServicesDir, constants.AWMRelayerInstallDir, constants.AWMRelayerConfigFilename),
		filepath.Join(cloudAWMRelayerConfigDir, constants.AWMRelayerConfigFilename),
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
		filepath.Join(constants.CloudNodeConfigPath, constants.GenesisFileName),
		constants.SSHFileOpsTimeout,
	); err != nil {
		return err
	}
	if err := host.Upload(
		filepath.Join(nodeInstanceDirPath, constants.NodeFileName),
		filepath.Join(constants.CloudNodeConfigPath, constants.NodeFileName),
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

func RunSSHUploadClustersConfig(host *models.Host, localClustersConfigPath string) error {
	remoteNodesDir := filepath.Join(constants.CloudNodeCLIConfigBasePath, constants.NodesDir)
	if err := host.MkdirAll(
		remoteNodesDir,
		constants.SSHDirOpsTimeout,
	); err != nil {
		return err
	}
	remoteClustersConfigPath := filepath.Join(remoteNodesDir, constants.ClustersConfigFileName)
	return host.Upload(
		localClustersConfigPath,
		remoteClustersConfigPath,
		constants.SSHFileOpsTimeout,
	)
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

// RunSSHExportSubnet exports deployed Subnet from local machine to cloud server
func RunSSHExportSubnet(host *models.Host, exportPath, cloudServerSubnetPath string) error {
	// name: copy exported subnet VM spec to cloud server
	return host.Upload(
		exportPath,
		cloudServerSubnetPath,
		constants.SSHFileOpsTimeout,
	)
}

// RunSSHTrackSubnet enables tracking of specified subnet
func RunSSHTrackSubnet(host *models.Host, subnetName, subnetAlias, importPath, networkFlag string) error {
	if _, err := host.Command(fmt.Sprintf("/home/ubuntu/bin/avalanche subnet import file %s --force", importPath), nil, constants.SSHScriptTimeout); err != nil {
		return err
	}
	if err := docker.StopDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	additionalFlags := ""
	if subnetAlias != "" {
		additionalFlags += fmt.Sprintf("--subnet-alias %s", subnetAlias)
	}
	if _, err := host.Command(fmt.Sprintf("/home/ubuntu/bin/avalanche subnet join %s %s --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write", subnetName, networkFlag), nil, constants.SSHScriptTimeout); err != nil {
		return err
	}
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout)
}

// RunSSHUpdateSubnet runs avalanche subnet join <subnetName> in cloud server using update subnet info
func RunSSHUpdateSubnet(host *models.Host, subnetName, importPath string) error {
	if err := docker.StopDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	if _, err := host.Command(fmt.Sprintf("/home/ubuntu/bin/avalanche subnet import file %s --force", importPath), nil, constants.SSHScriptTimeout); err != nil {
		return err
	}
	if _, err := host.Command(fmt.Sprintf("/home/ubuntu/bin/avalanche subnet join %s --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write", subnetName), nil, constants.SSHScriptTimeout); err != nil {
		return err
	}
	return docker.StartDockerComposeService(host, utils.GetRemoteComposeFile(), "avalanchego", constants.SSHLongRunningScriptTimeout)
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

// RunSSHSetupCLIFromSource installs any CLI branch from source
func RunSSHSetupCLIFromSource(host *models.Host, cliBranch string) error {
	if !constants.EnableSetupCLIFromSource {
		return nil
	}
	timeout := constants.SSHLongRunningScriptTimeout
	if utils.IsE2E() && utils.E2EDocker() {
		timeout = 10 * time.Minute
	}
	return RunOverSSH(
		"Setup CLI From Source",
		host,
		timeout,
		"shell/setupCLIFromSource.sh",
		scriptInputs{CliBranch: cliBranch},
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

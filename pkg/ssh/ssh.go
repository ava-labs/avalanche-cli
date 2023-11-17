// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"bytes"
	"embed"
	"fmt"
	"net/url"
	"path/filepath"
	"text/template"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

type scriptInputs struct {
	AvalancheGoVersion   string
	SubnetExportFileName string
	SubnetName           string
	GoVersion            string
	CliBranch            string
	IsDevNet             bool
	NetworkFlag          string
	SubnetEVMBinaryPath  string
	SubnetEVMReleaseURL  string
	SubnetEVMArchive     string
}

//go:embed shell/*.sh
var script embed.FS

// scriptLog formats the given line of a script log with the provided nodeID.
func scriptLog(nodeID string, line string) string {
	return fmt.Sprintf("[%s] %s", nodeID, line)
}

// RunOverSSH runs provided script path over ssh.
// This script can be template as it will be rendered using scriptInputs vars
func RunOverSSH(scriptDesc string, host models.Host, scriptPath string, templateVars scriptInputs) error {
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
	ux.Logger.PrintToUser(scriptLog(host.NodeID, scriptDesc))
	// make sure we are connected
	if err := host.Connect(constants.SSHScriptTimeout); err != nil {
		return err
	}
	if _, err := host.Command(script.String(), nil, host.Connection.Ctx); err != nil {
		return err
	}
	return nil
}

func PostOverSSH(host models.Host, path string, requestBody string) ([]byte, error) {
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
	// ignore response header
	_, responseBody, err := host.Forward(httpRequest)
	if err != nil {
		return nil, err
	}
	return responseBody, nil
}

// RunSSHSetupNode runs script to setup node
func RunSSHSetupNode(host models.Host, configPath, avalancheGoVersion string, isDevNet bool) error {
	if err := RunOverSSH("Setup Node", host, "shell/setupNode.sh", scriptInputs{AvalancheGoVersion: avalancheGoVersion, IsDevNet: isDevNet}); err != nil {
		return err
	}
	// name: copy metrics config to cloud server
	return host.Upload(configPath, filepath.Join(constants.CloudNodeConfigBasePath, filepath.Base(configPath)))
}

// RunSSHUpgradeAvalanchego runs script to upgrade avalanchego
func RunSSHUpgradeAvalanchego(host models.Host, avalancheGoVersion string) error {
	return RunOverSSH("Upgrade Avalanchego", host, "shell/upgradeAvalancheGo.sh", scriptInputs{AvalancheGoVersion: avalancheGoVersion})
}

// RunSSHStartNode runs script to start avalanchego
func RunSSHStartNode(host models.Host) error {
	return RunOverSSH("Start Avalanchego", host, "shell/startNode.sh", scriptInputs{})
}

// RunSSHStopNode runs script to stop avalanchego
func RunSSHStopNode(host models.Host) error {
	return RunOverSSH("Stop Avalanchego", host, "shell/stopNode.sh", scriptInputs{})
}

// RunSSHUpgradeSubnetEVM runs script to upgrade subnet evm
func RunSSHUpgradeSubnetEVM(host models.Host, subnetEVMBinaryPath string) error {
	return RunOverSSH("Upgrade Subnet EVM", host, "shell/upgradeSubnetEVM.sh", scriptInputs{SubnetEVMBinaryPath: subnetEVMBinaryPath})
}

// RunSSHGetNewSubnetEVMRelease runs script to download new subnet evm
func RunSSHGetNewSubnetEVMRelease(host models.Host, subnetEVMReleaseURL, subnetEVMArchive string) error {
	return RunOverSSH("Get Subnet EVM Release", host, "shell/getNewSubnetEVMRelease.sh", scriptInputs{SubnetEVMReleaseURL: subnetEVMReleaseURL, SubnetEVMArchive: subnetEVMArchive})
}

// RunSSHSetupDevNet runs script to setup devnet
func RunSSHSetupDevNet(host models.Host, nodeInstanceDirPath string) error {
	if err := host.MkdirAll(constants.CloudNodeConfigPath); err != nil {
		return err
	}
	if err := host.Upload(filepath.Join(nodeInstanceDirPath, constants.GenesisFileName), filepath.Join(constants.CloudNodeConfigPath, constants.GenesisFileName)); err != nil {
		return err
	}
	if err := host.Upload(filepath.Join(nodeInstanceDirPath, constants.NodeFileName), filepath.Join(constants.CloudNodeConfigPath, constants.NodeFileName)); err != nil {
		return err
	}
	// name: setup devnet
	return RunOverSSH("Setup DevNet", host, "shell/setupDevnet.sh", scriptInputs{})
}

// RunSSHUploadStakingFiles uploads staking files to a remote host via SSH.
func RunSSHUploadStakingFiles(host models.Host, nodeInstanceDirPath string) error {
	if err := host.MkdirAll(constants.CloudNodeStakingPath); err != nil {
		return err
	}
	if err := host.Upload(filepath.Join(nodeInstanceDirPath, constants.StakerCertFileName), filepath.Join(constants.CloudNodeStakingPath, constants.StakerCertFileName)); err != nil {
		return err
	}
	if err := host.Upload(filepath.Join(nodeInstanceDirPath, constants.StakerKeyFileName), filepath.Join(constants.CloudNodeStakingPath, constants.StakerKeyFileName)); err != nil {
		return err
	}
	return host.Upload(filepath.Join(nodeInstanceDirPath, constants.BLSKeyFileName), filepath.Join(constants.CloudNodeStakingPath, constants.BLSKeyFileName))
}

// RunSSHExportSubnet exports deployed Subnet from local machine to cloud server
func RunSSHExportSubnet(host models.Host, exportPath, cloudServerSubnetPath string) error {
	// name: copy exported subnet VM spec to cloud server
	return host.Upload(exportPath, cloudServerSubnetPath)
}

// RunSSHExportSubnet exports deployed Subnet from local machine to cloud server
// targets a specific host ansibleHostID in ansible inventory file
func RunSSHTrackSubnet(host models.Host, subnetName, importPath, networkFlag string) error {
	return RunOverSSH("Track Subnet", host, "shell/trackSubnet.sh", scriptInputs{SubnetName: subnetName, SubnetExportFileName: importPath, NetworkFlag: networkFlag})
}

// RunSSHUpdateSubnet runs avalanche subnet join <subnetName> in cloud server using update subnet info
func RunSSHUpdateSubnet(host models.Host, subnetName, importPath string) error {
	return RunOverSSH("Track Subnet", host, "shell/updateSubnet.sh", scriptInputs{SubnetName: subnetName, SubnetExportFileName: importPath})
}

// RunSSHSetupBuildEnv installs gcc, golang, rust and etc
func RunSSHSetupBuildEnv(host models.Host) error {
	return RunOverSSH("Setup Build Env", host, "shell/setupBuildEnv.sh", scriptInputs{GoVersion: constants.BuildEnvGolangVersion})
}

// RunSSHSetupCLIFromSource installs any CLI branch from source
func RunSSHSetupCLIFromSource(host models.Host, cliBranch string) error {
	return RunOverSSH("Setup CLI From Source", host, "shell/setupCLIFromSource.sh", scriptInputs{CliBranch: cliBranch})
}

// RunSSHCheckAvalancheGoVersion checks node avalanchego version
func RunSSHCheckAvalancheGoVersion(host models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.getNodeVersion\"}"
	return PostOverSSH(host, "", requestBody)
}

// RunSSHCheckBootstrapped checks if node is bootstrapped to primary network
func RunSSHCheckBootstrapped(host models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.isBootstrapped\", \"params\": {\"chain\":\"X\"}}"
	return PostOverSSH(host, "", requestBody)
}

// RunSSHCheckHealthy checks if node is healthy
func RunSSHCheckHealthy(host models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\":\"health.health\"}"
	return PostOverSSH(host, "/ext/health", requestBody)
}

// RunSSHGetNodeID reads nodeID from avalanchego
func RunSSHGetNodeID(host models.Host) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := "{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"info.getNodeID\"}"
	return PostOverSSH(host, "", requestBody)
}

// SubnetSyncStatus checks if node is synced to subnet
func RunSSHSubnetSyncStatus(host models.Host, blockchainID string) ([]byte, error) {
	// Craft and send the HTTP POST request
	requestBody := fmt.Sprintf("{\"jsonrpc\":\"2.0\", \"id\":1,\"method\" :\"platform.getBlockchainStatus\", \"params\": {\"blockchainID\":\"%s\"}}", blockchainID)
	return PostOverSSH(host, "/ext/bc/P", requestBody)
}

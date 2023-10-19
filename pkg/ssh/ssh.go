package ssh

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

type scriptInputs struct {
	AvalancheGoVersion   string
	SubnetExportFileName string
	SubnetName           string
	GoVersion            string
	CliBranch            string
}

//go:embed shell/*.sh
var script embed.FS

// RunSSHSetupNode runs provided script path over ssh.
// This script can be template as it will be rendered using scriptInputs vars
func RunOverSSH(id string, host models.Host, scriptPath string, templateVars scriptInputs) error {
	shellScript, err := script.ReadFile(scriptPath)
	if err != nil {
		return err
	}

	var script bytes.Buffer
	t, err := template.New(id).Parse(string(shellScript))
	if err != nil {
		return err
	}
	err = t.Execute(&script, templateVars)
	if err != nil {
		return err
	}
	return host.Command(script.String(), nil, context.Background())
}

func PostOverSSH(host models.Host, path string, requestBody string) ([]byte, error) {
	if path == "" {
		path = "/ext/info"
	}
	requestHeaders := fmt.Sprintf("POST %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Content-Length: %d\r\n"+
		"Content-Type: application/json\r\n\r\n", path, "127.0.0.1", len(requestBody))
	httpRequest := requestHeaders + requestBody
	//ignore response header
	_, responseBody, err := host.Forward(httpRequest)
	if err != nil {
		return nil, err
	}
	return responseBody,nil
}

// RunSSHSetupNode runs script to setup node
func RunSSHSetupNode(host models.Host, configPath, avalancheGoVersion string) error {
	// name: setup node
	if err := RunOverSSH("SetupNode", host, "shell/setupNode.sh", scriptInputs{AvalancheGoVersion: avalancheGoVersion}); err != nil {
		return err
	}
	// name: copy metrics config to cloud server
	if err := host.Upload(configPath, fmt.Sprintf("/home/ubuntu/.avalanche-cli/%s", filepath.Base(configPath))); err != nil {
		return err
	}
	return nil
}

func RunSSHCopyStakingFiles(host models.Host, configPath, nodeInstanceDirPath string) error {
	// name: copy staker.crt to local machine
	if err := host.Download("/home/ubuntu/.avalanchego/staking/staker.crt", fmt.Sprintf("%s/staker.crt", nodeInstanceDirPath)); err != nil {
		return err
	}
	// name: copy staker.key to local machine
	if err := host.Download("/home/ubuntu/.avalanchego/staking/staker.key", fmt.Sprintf("%s/staker.key", nodeInstanceDirPath)); err != nil {
		return err
	}
	// name: copy signer.key to local machine
	if err := host.Download("/home/ubuntu/.avalanchego/staking/signer.key", fmt.Sprintf("%s/signer.key", nodeInstanceDirPath)); err != nil {
		return err
	}
	return nil
}

// RunSSHExportSubnet exports deployed Subnet from local machine to cloud server
func RunSSHExportSubnet(host models.Host, exportPath, cloudServerSubnetPath string) error {
	// name: copy exported subnet VM spec to cloud server
	return host.Upload(exportPath, cloudServerSubnetPath)
}

// RunSSHExportSubnet exports deployed Subnet from local machine to cloud server
// targets a specific host ansibleHostID in ansible inventory file
func RunSSHTrackSubnet(host models.Host, subnetName, importPath string) error {
	return RunOverSSH("TrackSubnet", host, "shell/trackSubnet.sh", scriptInputs{SubnetName: subnetName, SubnetExportFileName: importPath})
}

// RunSSHUpdateSubnet runs avalanche subnet join <subnetName> in cloud server using update subnet info
func RunSSHUpdateSubnet(host models.Host, subnetName, importPath string) error {
	return RunOverSSH("TrackSubnet", host, "shell/updateSubnet.sh", scriptInputs{SubnetName: subnetName, SubnetExportFileName: importPath})
}

// RunSSHSetupBuildEnv installs gcc, golang, rust and etc
func RunSSHSetupBuildEnv(host models.Host) error {
	return RunOverSSH("setupBuildEnv", host, "shell/setupBuildEnv.sh", scriptInputs{GoVersion: constants.BuildEnvGolangVersion})
}

// RunSSHSetupCLIFromSource installs any CLI branch from source
func RunSSHSetupCLIFromSource(host models.Host, cliBranch string) error {
	return RunOverSSH("setupCLIFromSource", host, "shell/setupCLIFromSource.sh", scriptInputs{CliBranch: cliBranch})
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

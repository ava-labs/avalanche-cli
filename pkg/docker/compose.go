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
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

type dockerComposeInputs struct {
	WithMonitoring     bool
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

func pushComposeFile(host *models.Host, localFile string, remoteFile string) error {
	if !utils.FileExists(localFile) {
		return fmt.Errorf("file %s does not exist", localFile)
	}
	if err := host.MkdirAll(filepath.Dir(remoteFile), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	if err := host.Upload(localFile, remoteFile, constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	return nil
}

func StartDockerCompose(host *models.Host, composeFile string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s up -d --wait --pull missing", composeFile), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func StopDockerCompose(host *models.Host, composeFile string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s down", composeFile), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func RestartDockerCompose(host *models.Host, composeFile string, timeout time.Duration) error {
	if err := StopDockerCompose(host, composeFile, timeout); err != nil {
		return err
	}
	return StartDockerCompose(host, composeFile, timeout)
}

func StartDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s start %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func StopDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s stop %s", composeFile, service), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func RestartDockerComposeService(host *models.Host, composeFile string, service string, timeout time.Duration) error {
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s restart %s", composeFile, service), nil, timeout); err != nil {
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
	if err := pushComposeFile(host, tmpFile.Name(), remoteComposeFile); err != nil {
		return err
	}
	if err := StartDockerCompose(host, remoteComposeFile, timeout); err != nil {
		return err
	}
	executionTime := time.Since(startTime)
	ux.Logger.Info("ComposeOverSSH[%s]%s took %s with err: %v", host.NodeID, composeDesc, executionTime, err)
	return nil
}

// ListRemoteComposeServices lists the services in a remote docker-compose file.
func ListRemoteComposeServices(host *models.Host, composeFile string, timeout time.Duration) ([]string, error) {
	output, err := host.Command(fmt.Sprintf("docker-compose -f %s config --services", composeFile), nil, timeout)
	if err != nil {
		return nil, err
	}
	return utils.CleanupStrings(utils.SplitSeparatedBytesToString(output, "\n")), nil
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

// ComposeSSHSetupNode sets up an AvalancheGo node and dependencies on a remote host over SSH.
func ComposeSSHSetupNode(host *models.Host, avalancheGoVersion string, withMonitoring bool) error {
	return ComposeOverSSH("Setup Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		dockerComposeInputs{
			AvalanchegoVersion: avalancheGoVersion,
			WithMonitoring:     withMonitoring,
		})
}

// WasNodeSetupWithMonitoring checks if an AvalancheGo node was setup with monitoring on a remote host.
func WasNodeSetupWithMonitoring(host *models.Host) (bool, error) {
	return HasRemoteComposeService(host, utils.GetRemoteComposeFile(), "promtail", constants.SSHScriptTimeout)
}

// ComposeSSHSetupCChain sets up an Avalanche C-Chain node and dependencies on a remote host over SSH.
func ComposeSSHSetupMonitoring(host *models.Host) error {
	return ComposeOverSSH("Setup Monitoring",
		host,
		constants.SSHScriptTimeout,
		"templates/monitoring.docker-compose.yml",
		dockerComposeInputs{})
}

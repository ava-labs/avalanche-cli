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
	IsMonitoringEnabled bool
	AvalancheGoVersion  string
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
	if output, err := host.Command(fmt.Sprintf("docker-compose -f %s up -d", composeFile), nil, timeout); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func ComposeOverSSH(
	composeDesc string,
	host *models.Host,
	timeout time.Duration,
	composePath string,
	composeVars dockerComposeInputs,
) error {
	remoteComposeFile := filepath.Join(constants.CloudNodeCLIConfigBasePath, "docker-compose.yml")
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

func ComposeSSHSetupNode(host *models.Host, avalancheGoVersion string, withMonitoring bool) error {
	return ComposeOverSSH("Setup Node",
		host,
		constants.SSHScriptTimeout,
		"templates/avalanchego.docker-compose.yml",
		dockerComposeInputs{
			AvalancheGoVersion:  avalancheGoVersion,
			IsMonitoringEnabled: withMonitoring,
		})
}

func ComposeSSHSetupMonitoring(host *models.Host) error {
	return ComposeOverSSH("Setup Monitoring",
		host,
		constants.SSHScriptTimeout,
		"templates/monitoring.docker-compose.yml",
		dockerComposeInputs{})
}

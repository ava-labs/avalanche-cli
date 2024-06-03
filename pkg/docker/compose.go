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
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

type dockerComposeInputs struct {
	WithMonitoring     bool
	WithAvalanchego    bool
	AvalanchegoVersion string
	E2E                bool
	E2EIP              string
	E2ESuffix          string
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
		return fmt.Errorf("file %s does not exist to be uploaded to host: %s", localFile, host.NodeID)
	}
	if err := host.MkdirAll(filepath.Dir(remoteFile), constants.SSHFileOpsTimeout); err != nil {
		return err
	}
	fileExists, err := host.FileExists(remoteFile)
	if err != nil {
		return err
	}
	ux.Logger.Info("Pushing compose file %s to %s:%s", localFile, host.NodeID, remoteFile)
	if fileExists && merge {
		// upload new and merge files
		ux.Logger.Info("Merging compose files")
		tmpFile, err := host.CreateTempFile()
		if err != nil {
			return err
		}
		defer func() {
			if err := host.Remove(tmpFile, false); err != nil {
				ux.Logger.Error("Error removing temporary file %s:%s %s", host.NodeID, tmpFile, err)
			}
		}()
		if err := host.Upload(localFile, tmpFile, constants.SSHFileOpsTimeout); err != nil {
			return err
		}
		if err := mergeComposeFiles(host, remoteFile, tmpFile); err != nil {
			return err
		}
	} else {
		ux.Logger.Info("Uploading compose file for host; %s", host.NodeID)
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
	// we provide systemd service unit for docker compose if the host has systemd
	if host.IsSystemD() {
		if output, err := host.Command("sudo systemctl start avalanche-cli-docker", nil, timeout); err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
	} else {
		composeFile := utils.GetRemoteComposeFile()
		output, err := host.Command(fmt.Sprintf("docker compose -f %s up -d", composeFile), nil, constants.SSHScriptTimeout)
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
	}
	return nil
}

func StopDockerCompose(host *models.Host, timeout time.Duration) error {
	if host.IsSystemD() {
		if output, err := host.Command("sudo systemctl stop avalanche-cli-docker", nil, timeout); err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
	} else {
		composeFile := utils.GetRemoteComposeFile()
		output, err := host.Command(fmt.Sprintf("docker compose -f %s down", composeFile), nil, constants.SSHScriptTimeout)
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
	}
	return nil
}

func RestartDockerCompose(host *models.Host, timeout time.Duration) error {
	if host.IsSystemD() {
		if output, err := host.Command("sudo systemctl restart avalanche-cli-docker", nil, timeout); err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
	} else {
		composeFile := utils.GetRemoteComposeFile()
		output, err := host.Command(fmt.Sprintf("docker compose -f %s restart", composeFile), nil, constants.SSHScriptTimeout)
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(output))
		}
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
	ux.Logger.Info("ValidateComposeFile [%s]%s", host.NodeID, composeDesc)
	if err := ValidateComposeFile(host, remoteComposeFile, timeout); err != nil {
		ux.Logger.Error("ComposeOverSSH[%s]%s failed to validate: %v", host.NodeID, composeDesc, err)
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
	return utils.CleanupStrings(strings.Split(string(output), "\n")), nil
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

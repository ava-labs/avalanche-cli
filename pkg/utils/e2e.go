// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

const composeTemplate = `version: '3'
services:
{{- $version := .UbuntuVersion }}
{{- $pubkey := .SSHPubKey }}
{{- range $i, $ip := .IPs }}
  ubuntu{{$i}}:
    privileged: true
    image: ubuntu:{{$version}}
    container_name: ubuntu_container{{$i}}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:rw
    networks:
      e2e:
        ipv4_address: {{$ip}}
    command: >
	    /bin/bash -c "export DEBIAN_FRONTEND=noninteractive; set -e; sshd -V || apt-get update && apt-get install -y sudo openssh-server;
		  id ubuntu || useradd -m -s /bin/bash ubuntu; mkdir -p /home/ubuntu/.ssh;
		  echo '{{$pubkey}}' | base64 -d > /home/ubuntu/.ssh/authorized_keys; chown -R ubuntu:sudo /home/ubuntu/.ssh; echo 'ubuntu ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers;
		  service ssh start && tail -f /dev/null"
{{- end }}
networks:
  e2e:
    ipam:
      driver: default
      config:
        - subnet: {{.NetworkPrefix}}.0/16
`

// Config holds the information needed for the template
type Config struct {
	IPs           []string
	UbuntuVersion string
	NetworkPrefix string
	SSHPubKey     string
}

// IsE2E checks if the environment variable "RUN_E2E" is set and returns true if it is, false otherwise.
func IsE2E() bool {
	return os.Getenv("RUN_E2E") != ""
}

// E2EDocker checks if docker and docker-compose are available.
func E2EDocker() bool {
	cmd := exec.Command("docker-compose", "--version")
	cmd.Env = os.Environ()
	err := cmd.Run()
	return err == nil
}

// GenDockerComposeFile generates a Docker Compose file with the specified number of nodes and Ubuntu version.
func GenDockerComposeFile(nodes int, ubuntuVersion string, networkPrefix string, sshPubKey string) (string, error) {
	var ips []string
	for i := 1; i <= nodes; i++ {
		ips = append(ips, fmt.Sprintf("%s.%d", networkPrefix, i+1))
	}
	config := Config{
		IPs:           ips,
		UbuntuVersion: ubuntuVersion,
		NetworkPrefix: networkPrefix,
		SSHPubKey:     base64.StdEncoding.EncodeToString([]byte(sshPubKey)),
	}
	tmpl, err := template.New("docker-compose").Parse(strings.ReplaceAll(composeTemplate, "\t", "  "))
	if err != nil {
		return "", err
	}
	var result bytes.Buffer
	writer := &result
	err = tmpl.Execute(writer, config)
	if err != nil {
		fmt.Println("Error executing Docker Compose template:", err)
		return "", err
	}
	return result.String(), nil
}

// SaveDockerComposeFile saves the Docker Compose file with the specified number of nodes and Ubuntu version.
func SaveDockerComposeFile(fileName string, nodes int, ubuntuVersion string, sshPubKey string) (string, error) {
	var (
		tmpFile *os.File
		err     error
	)
	if fileName != "" {
		tmpFile, err = os.Create(fileName)
		if err != nil {
			return "", fmt.Errorf("error creating file %s: %w", fileName, err)
		}
	} else {
		tmpFile, err = os.CreateTemp("", "docker-compose-*.yml")
		if err != nil {
			return "", fmt.Errorf("error creating temporary file: %w", err)
		}
	}
	composeFile, err := GenDockerComposeFile(nodes, ubuntuVersion, constants.E2ENetworkPrefix, sshPubKey)
	fmt.Println("Generated Docker Compose file:", composeFile)
	if err != nil {
		return "", fmt.Errorf("error generating Docker Compose file: %w", err)
	}
	if err := os.WriteFile(tmpFile.Name(), []byte(composeFile), 0o600); err != nil {
		return "", fmt.Errorf("error writing temporary file: %w", err)
	}
	return tmpFile.Name(), nil
}

// StartDockerCompose is a function that starts Docker Compose.
func StartDockerCompose(filePath string) error {
	cmd := exec.Command("docker", "compose", "-f", filePath, "up", "--detach", "--remove-orphans")
	fmt.Println("Starting Docker Compose... with command:", cmd.String())
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StopDockerCompose stops the Docker Compose services defined in the specified file.
func StopDockerCompose(filePath string) error {
	cmd := exec.Command("docker-compose", "-f", filePath, "down")
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GenerateDockerHostIDs generates a list of Docker host IDs.
func GenerateDockerHostIDs(numNodes int) []string {
	var ids []string
	for i := 1; i <= numNodes; i++ {
		ids = append(ids, fmt.Sprintf("docker%d-%s", i, RandomString(5)))
	}
	return ids
}

func GenerateDockerHostIPs(numNodes int) []string {
	var ips []string
	for i := 1; i <= numNodes; i++ {
		ips = append(ips, fmt.Sprintf("%s.%d", constants.E2ENetworkPrefix, i+1))
	}
	return ips
}

func RemoveLineCleanChars(s string) string {
	re := regexp.MustCompile(`\r\x1b\[K`)
	return re.ReplaceAllString(s, "")
}

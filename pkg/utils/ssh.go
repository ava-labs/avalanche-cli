// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/sdk/utils"

	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/exp/slices"
)

// GetSSHConnectionString returns the SSH connection string for the given public IP and certificate file path.
func GetSSHConnectionString(publicIP, certFilePath string) string {
	if certFilePath != "" {
		certFilePath = fmt.Sprintf("-i %s", certFilePath)
	}
	return fmt.Sprintf("ssh %s %s@%s %s", constants.AnsibleSSHShellParams, constants.RemoteSSHUser, publicIP, certFilePath)
}

// GetSCPTargetPath returns the target path for the given source path and target directory.
func GetSCPTargetPath(ip, path string) string {
	if ip == "" {
		return path
	}
	return fmt.Sprintf("%s@%s:%s", constants.RemoteSSHUser, ip, path)
}

// GetSCPCommandString returns the SCP command string for the given source and destination paths.
func GetSCPCommandString(certFilePath string, sourceIP, sourcePath string, destIP, destPath string, recursive, withCompression bool) (string, error) {
	scpParams := constants.AnsibleSSHShellParams + " -B -o LogLevel=Error"
	if sourceIP == "" && destIP == "" {
		return "", fmt.Errorf("source or destination should be remote")
	}
	if sourcePath == "" || destPath == "" {
		return "", fmt.Errorf("source and destination path are required")
	}
	// end of checks
	if recursive {
		scpParams += " -r"
	}
	if withCompression {
		scpParams += " -C"
	}
	if certFilePath != "" {
		scpParams += fmt.Sprintf(" -i %s", certFilePath)
	}
	if sourceIP != "" && destIP != "" {
		scpParams += " -3"
	}
	if sourceIP != "" {
		sourcePath = GetSCPTargetPath(sourceIP, sourcePath)
	}
	if destIP != "" {
		destPath = GetSCPTargetPath(destIP, destPath)
	}

	return fmt.Sprintf("scp %s %s %s", scpParams, sourcePath, destPath), nil
}

// SplitSCPPath splits the given path into host and path.
func SplitSCPPath(path string) (string, string) {
	if !strings.Contains(path, ":") {
		return "", path
	}
	parts := strings.Split(path, ":")
	return parts[0], parts[1]
}

// CombineSCPPath combines the given host and path into a single item for scp.
func CombineSCPPath(host, path string) string {
	if host != "" {
		return fmt.Sprintf("%s:%s", host, path)
	}
	return path
}

// IsSSHAgentAvailable checks if the SSH agent is available.
func IsSSHAgentAvailable() bool {
	return os.Getenv("SSH_AUTH_SOCK") != ""
}

func getSSHAgent() (agent.ExtendedAgent, error) {
	if !IsSSHAgentAvailable() {
		return nil, fmt.Errorf("SSH agent is not available")
	}
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, fmt.Errorf("SSH agent is not accepting connections: %w", err)
	}
	sshAgent := agent.NewClient(conn)
	return sshAgent, nil
}

// ListSSHAgentIdentities returns a list of SSH identities from ssh-agent.
func ListSSHAgentIdentities() ([]string, error) {
	sshAgent, err := getSSHAgent()
	if err != nil {
		return nil, err
	}
	sshIDs, err := sshAgent.List()
	if err != nil {
		return nil, err
	}
	identityList := utils.Map(sshIDs, func(id *agent.Key) string { return id.Comment })
	return identityList, nil
}

func IsSSHAgentIdentityValid(identity string) (bool, error) {
	identityList, err := ListSSHAgentIdentities()
	if err != nil {
		return false, err
	}
	return slices.Contains(identityList, identity), nil
}

func ReadSSHAgentIdentityPublicKey(identityName string) (string, error) {
	identityValid, err := IsSSHAgentIdentityValid(identityName)
	if err != nil {
		return "", err
	}
	if !identityValid {
		return "", fmt.Errorf("identity %s not found", identityName)
	}
	sshAgent, err := getSSHAgent()
	if err != nil {
		return "", err
	}
	sshIDs, err := sshAgent.List()
	if err != nil {
		return "", err
	}
	for _, id := range sshIDs {
		if id.Comment == identityName {
			// Retrieve the public key for the matched identity
			return id.String(), nil
		}
	}
	return "", fmt.Errorf("identity %s can't be read", identityName)
}

// IsSSHPubKey checks if the given string is a valid SSH public key.
func IsSSHPubKey(pubkey string) bool {
	key := strings.Trim(pubkey, "\"'")
	// Regular expression pattern to match SSH public key
	pattern := `^(ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp256)\s[A-Za-z0-9+/]+[=]{0,3}(\s+[^\s]+)?$`

	// Compile the regular expression
	regex := regexp.MustCompile(pattern)

	// Check if the key matches the pattern
	return regex.MatchString(key)
}

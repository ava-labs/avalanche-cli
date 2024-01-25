// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"golang.org/x/crypto/ssh/agent"
)

// GetSSHConnectionString returns the SSH connection string for the given public IP and certificate file path.
func GetSSHConnectionString(publicIP, certFilePath string) string {
	if certFilePath != "" {
		return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSSHShellParams, constants.AnsibleSSHUser, publicIP, certFilePath)
	} else {
		return fmt.Sprintf("ssh %s %s@%s", constants.AnsibleSSHUseAgentParams, constants.AnsibleSSHUser, publicIP)
	}
}

// isSSHAgentAvailable checks if the SSH agent is available.
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

// ListSSHAgentIdentity returns a list of SSH identities from ssh-agent.
func ListSSHAgentIdentities() ([]string, error) {
	sshAgent, err := getSSHAgent()
	if err != nil {
		return nil, err
	}
	sshIDs, err := sshAgent.List()
	if err != nil {
		return nil, err
	}
	identityList := Map(sshIDs, func(id *agent.Key) string { return id.Comment })
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

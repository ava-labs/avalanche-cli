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
	return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSSHShellParams, constants.AnsibleSSHUser, publicIP, certFilePath)

}

// isSSHAgentAvailable checks if the SSH agent is available.
func IsSSHAgentAvailable() bool {
	return os.Getenv("SSH_AUTH_SOCK") != ""

}

func GetSSHAgent() (agent.ExtendedAgent, error) {
	if !IsSSHAgentAvailable() {
		return nil, fmt.Errorf("SSH agent is not available")
	}
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, err
	}
	sshAgent := agent.NewClient(conn)
	return sshAgent, nil
}

// ListSSHIdentity returns a list of SSH identities and any error encountered.
func ListSSHIdentity() ([]string, error) {
	sshAgent, err := GetSSHAgent()
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

func IsSSHIdentityValid(identity string) bool {
	identityList, err := ListSSHIdentity()
	if err != nil {
		return false
	}
	return slices.Contains(identityList, identity)
}

func ReadSSHIdentityPublicKey(identityName string) (string, error) {
	if !IsSSHIdentityValid(identityName) {
		return "", fmt.Errorf("identity %s not found", identityName)
	}
	sshAgent, err := GetSSHAgent()
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

type Host struct {
	NodeID            string
	IP                string
	SSHUser           string
	SSHPrivateKeyPath string
	SSHCommonArgs     string
}

func (h Host) GetAnsibleInventoryRecord() string {
	return strings.Join([]string{
		h.NodeID,
		fmt.Sprintf("ansible_host=%s", h.IP),
		fmt.Sprintf("ansible_user=%s", h.SSHUser),
		fmt.Sprintf("ansible_ssh_private_key_file=%s", h.SSHPrivateKeyPath),
		fmt.Sprintf("ansible_ssh_common_args='%s'", h.SSHCommonArgs),
	}, " ")
}

func HostCloudIDToAnsibleID(cloudService string, hostCloudID string) (string, error) {
	switch cloudService {
	case constants.GCPCloudService:
		return fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, hostCloudID), nil
	case constants.AWSCloudService:
		return fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, hostCloudID), nil
	}
	return "", fmt.Errorf("unknown cloud service %s", cloudService)
}

func HostAnsibleIDToCloudID(hostAnsibleID string) (string, string, error) {
	if strings.HasPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix) {
		return constants.AWSCloudService, strings.TrimPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix+"_"), nil
	} else if strings.HasPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix) {
		return constants.GCPCloudService, strings.TrimPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix+"_"), nil
	}
	return "", "", fmt.Errorf("unknown cloud service prefix in %s", hostAnsibleID)
}

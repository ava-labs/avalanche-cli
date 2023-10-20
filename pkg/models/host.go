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

func (h Host) GetNodeID() string {
	return strings.TrimPrefix(h.NodeID, constants.AnsibleAWSNodePrefix)
}

func (h Host) ConvertToNodeID(nodeName string) string {
	h = Host{
		NodeID:            nodeName,
		SSHUser:           "ubuntu",
		SSHPrivateKeyPath: "",
		SSHCommonArgs:     "",
	}
	return h.GetNodeID()
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

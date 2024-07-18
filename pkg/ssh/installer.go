// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

type HostInstaller struct {
	Host *models.Host
}

func NewHostInstaller(host *models.Host) *HostInstaller {
	return &HostInstaller{Host: host}
}

func (h *HostInstaller) GetArch() (string, string) {
	goArhBytes, err := h.Host.Command("go env GOARCH", nil, constants.SSHScriptTimeout)
	if err != nil {
		return "", ""
	}
	goOSBytes, err := h.Host.Command("go env GOOS", nil, constants.SSHScriptTimeout)
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(string(goArhBytes)), strings.TrimSpace(string(goOSBytes))
}

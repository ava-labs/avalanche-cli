// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import (
	"embed"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

//go:embed templates/*
var templates embed.FS

// RemoteFoldersToCreate returns a list of folders that need to be created on the remote machine
func RemoteFoldersToCreate() []string {
	return utils.AppendSlices[string](
		GrafanaFoldersToCreate(),
		LokiFoldersToCreate(),
		PrometheusFoldersToCreate(),
		PromtailFoldersToCreate(),
	)
}

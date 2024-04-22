// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package remoteconfig

import "github.com/ava-labs/avalanche-cli/pkg/utils"

func PrometheusFoldersToCreate() []string {
	return []string{utils.GetRemoteComposeServicePath("prometheus")}
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetSSHConnectionString(publicIP, certFilePath string) string {
	return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSSHShellParams, constants.AnsibleSSHUser, publicIP, certFilePath)
}

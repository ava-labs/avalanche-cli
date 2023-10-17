// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetSSHConnectionString(params, publicIP, certFilePath string) string {
	if params == "" {
		params = constants.AnsibleSSHParams
	}
	return fmt.Sprintf("ssh %s %s %s@%s -i %s", constants.AnsibleSSHParams, params, constants.AnsibleSSHUser, publicIP, certFilePath)
}

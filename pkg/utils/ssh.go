// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetSSHConnectionString(params, publicIP, certFilePath string) string {
	if params == "" {
		params = constants.AnsibleSSHParams
	}
	return fmt.Sprintf("ssh %s %s@%s -i %s", params, constants.AnsibleSSHUser, publicIP, certFilePath)
}

func getStringSeqFromISeq(lines []interface{}) []string {
	seq := []string{}
	for _, lineI := range lines {
		line, ok := lineI.(string)
		if ok {
			if strings.Contains(line, "Usage:") {
				break
			}
			seq = append(seq, line)
		}
	}
	return seq
}

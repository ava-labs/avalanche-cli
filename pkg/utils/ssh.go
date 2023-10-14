// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func GetSSHConnectionString(publicIP, certFilePath string) string {
	return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSSHParams, constants.AnsibleSSHUser, publicIP, certFilePath)
}

func DisplayErrMsg(buffer *bytes.Buffer) error {
	for _, line := range strings.Split(buffer.String(), "\n") {
		if strings.Contains(line, "FAILED") || strings.Contains(line, "UNREACHABLE") {
			i := strings.Index(line, "{")
			if i >= 0 {
				line = line[i:]
			}
			var jsonMap map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonMap); err != nil {
				return err
			}
			toDump := []string{}
			stdoutLines, ok := jsonMap["stdout_lines"].([]interface{})
			if ok {
				toDump = append(toDump, getStringSeqFromISeq(stdoutLines)...)
			}
			stderrLines, ok := jsonMap["stderr_lines"].([]interface{})
			if ok {
				toDump = append(toDump, getStringSeqFromISeq(stderrLines)...)
			}
			msgLine, ok := jsonMap["msg"].(string)
			if ok {
				toDump = append(toDump, msgLine)
			}
			contentLine, ok := jsonMap["content"].(string)
			if ok {
				toDump = append(toDump, contentLine)
			}
			if len(toDump) > 0 {
				fmt.Println()
				fmt.Println(logging.Red.Wrap("Message from cloud node:"))
				for _, l := range toDump {
					fmt.Println("  " + logging.Red.Wrap(l))
				}
				fmt.Println()
			}
		}
	}
	return nil
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

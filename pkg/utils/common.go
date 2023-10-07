// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
)

func SetupRealtimeCLIOutput(cmd *exec.Cmd, redirectStdout bool, redirectStderr bool) (*bytes.Buffer, *bytes.Buffer) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	if redirectStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuffer)
	} else {
		cmd.Stdout = io.MultiWriter(&stdoutBuffer)
	}
	if redirectStderr {
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)
	} else {
		cmd.Stderr = io.MultiWriter(&stderrBuffer)
	}
	return &stdoutBuffer, &stderrBuffer
}

func SplitKeyValueStringToMap(str string, delimiter string) map[string]string {
	kvMap := make(map[string]string)
	if str == "" {
		return kvMap
	}
	entries := strings.Split(str, delimiter)
	for _, e := range entries {
		parts := strings.Split(e, "=")
		if len(parts) >= 2 {
			kvMap[parts[0]] = parts[1]
		} else {
			kvMap[parts[0]] = parts[0]
		}
	}
	return kvMap
}

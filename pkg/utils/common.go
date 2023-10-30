// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/melbahja/goph"
)

// SetupRealtimeCLIOutput sets up the real-time CLI output for a given command.
//
// It takes the following parameters:
// - cmd: a pointer to the exec.Cmd struct representing the command to be executed.
// - redirectStdout: a boolean indicating whether to redirect the standard output of the command.
// - redirectStderr: a boolean indicating whether to redirect the standard error of the command.
//
// It returns two pointers to bytes.Buffer structs: one for capturing the standard output
// and one for capturing the standard error.
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

func SetupRealtimeCLISSHOutput(cmd *goph.Cmd, redirectStdout bool) *bytes.Buffer {
	var stdoutBuffer bytes.Buffer
	if redirectStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuffer)
	} else {
		cmd.Stdout = io.MultiWriter(&stdoutBuffer)
	}
	return &stdoutBuffer
}

// SplitKeyValueStringToMap splits a string with multiple key-value pairs separated by delimiter.
// Delimiter must be a single character
func SplitKeyValueStringToMap(str string, delimiter string) (map[string]string, error) {
	kvMap := make(map[string]string)
	if str == "" || len(delimiter) != 1 {
		return kvMap, nil
	}
	entries := SplitStringWithQuotes(str, rune(delimiter[0]))
	for _, e := range entries {
		parts := strings.Split(e, "=")
		if len(parts) >= 2 {
			kvMap[parts[0]] = strings.Trim(strings.Join(parts[1:], "="), "'")
		} else {
			kvMap[parts[0]] = strings.Trim(parts[0], "'")
		}
	}
	return kvMap, nil
}

// SplitString split string with a rune comma ignore quoted
func SplitStringWithQuotes(str string, r rune) []string {
	quoted := false
	return strings.FieldsFunc(str, func(r1 rune) bool {
		if r1 == '\'' {
			quoted = !quoted
		}
		return !quoted && r1 == r
	})
}

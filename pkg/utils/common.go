// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
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

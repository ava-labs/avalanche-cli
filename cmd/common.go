// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/docker/docker/pkg/reexec"
	"github.com/shirou/gopsutil/process"
)

const (
	procName = "backend start"
)

type ProcessChecker interface {
	// IsServerProcessRunning returns true if the gRPC server is running,
	// or false if not
	IsServerProcessRunning() (bool, error)
}

type realProcessRunner struct{}

func NewProcessChecker() ProcessChecker {
	return &realProcessRunner{}
}

// IsServerProcessRunning returns true if the gRPC server is running,
// or false if not
func (rpr *realProcessRunner) IsServerProcessRunning() (bool, error) {
	// get OS process list
	procs, err := process.Processes()
	if err != nil {
		return false, err
	}

	// iterate all processes...
	for _, p := range procs {
		name, err := p.Cmdline()
		if err != nil {
			return false, err
		}
		// ... and string-compare
		// TODO is there a better way to do this?
		if strings.Contains(name, procName) {
			return true, nil
		}
	}
	return false, nil
}

// start the gRPC server as a reentrant process of this binary
// it just executes `avalanche-cli backend start`
func startServerProcess() error {
	thisBin := reexec.Self()

	args := []string{"backend", "start"}
	cmd := exec.Command(thisBin, args...)
	outputFile, err := os.CreateTemp("", "avalanche-cli-backend*")
	if err != nil {
		return err
	}
	// TODO: should this be redirected to this app's log file instead?
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Start(); err != nil {
		return err
	}

	log.Info("Backend controller started, pid: %d, output at: %s", cmd.Process.Pid, outputFile.Name())
	content := fmt.Sprintf("gRPC server output file: %s\ngRPC server PID: %d\n", outputFile.Name(), cmd.Process.Pid)
	err = os.WriteFile(serverRun, []byte(content), perms.ReadWrite)
	if err != nil {
		log.Warn("could not write gRPC process info to file: %s", err)
	}
	return nil
}

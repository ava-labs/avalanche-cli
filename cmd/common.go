// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/docker/docker/pkg/reexec"
	"github.com/shirou/gopsutil/process"
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

func NewGRPCClient(logLevel string, endpoint string, timeout time.Duration) (client.Client, error) {
	return client.New(client.Config{
		LogLevel:    logLevel,
		Endpoint:    endpoint,
		DialTimeout: timeout,
	})
}

// IsServerProcessRunning returns true if the gRPC server is running,
// or false if not
func (rpr *realProcessRunner) IsServerProcessRunning() (bool, error) {
	pid, err := GetServerPID()
	if err != nil {
		return false, err
	}

	// get OS process list
	procs, err := process.Processes()
	if err != nil {
		return false, err
	}

	p32 := int32(pid)
	// iterate all processes...
	for _, p := range procs {
		if p.Pid == p32 {
			return true, nil
		}
	}
	return false, nil
}

func GetServerPID() (int, error) {
	runFile, err := os.ReadFile(serverRun)
	if err != nil {
		return 0, fmt.Errorf("failed reading process info file at %s: %s\n", serverRun, err)
	}
	str := string(runFile)
	pidIndex := strings.Index(str, "PID:")
	pidStart := pidIndex + len("PID: ")
	pidstr := str[pidStart:strings.LastIndex(str, "\n")]
	pid, err := strconv.Atoi(strings.TrimSpace(pidstr))
	if err != nil {
		return 0, fmt.Errorf("failed reading pid from info file at %s: %s\n", serverRun, err)
	}
	return pid, nil
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

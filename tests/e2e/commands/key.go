// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commands

import (
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func CreateKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		keyName,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	out, err := cmd.Output()
	return string(out), err
}

func CreateKeyFromPath(keyName string, keyPath string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		"--file",
		keyPath,
		keyName,
		"--skip-balances",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204
	out, err := cmd.Output()
	return string(out), err
}

func CreateKeyForce(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		keyName,
		"--force",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	out, err := cmd.Output()
	return string(out), err
}

func ListKeys(network string, omitCChain bool, useNanoAvax bool) (string, error) {
	args := []string{KeyCmd, "list", "--" + network, "--" + constants.SkipUpdateFlag}
	if omitCChain {
		args = append(args, "--cchain=false")
	}
	if useNanoAvax {
		args = append(args, "--use-nano-avax=true")
	}
	cmd := exec.Command(CLIBinary, args...)
	out, err := cmd.Output()
	return string(out), err
}

func DeleteKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"delete",
		keyName,
		"--force",
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	out, err := cmd.Output()
	return string(out), err
}

func ExportKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"export",
		keyName,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	out, err := cmd.Output()
	return string(out), err
}

func ExportKeyToFile(keyName string, outputPath string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"export",
		keyName,
		"-o",
		outputPath,
		"--"+constants.SkipUpdateFlag,
	) // #nosec G204

	out, err := cmd.Output()
	return string(out), err
}

func KeyTransferSend(keyName string, targetAddr string, amount string) (string, error) {
	// Create config
	args := []string{
		KeyCmd,
		"transfer",
		"--local",
		"--key",
		keyName,
		"--send",
		"--destination-addr",
		targetAddr,
		"--amount",
		amount,
		"--fund-p-chain",
		"--force",
		"--" + constants.SkipUpdateFlag,
	}
	cmd := exec.Command(CLIBinary, args...)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func KeyTransferReceive(keyName string, amount string, recoveryStep string) (string, error) {
	// Create config
	args := []string{
		KeyCmd,
		"transfer",
		"--local",
		"--key",
		keyName,
		"--receive",
		"--amount",
		amount,
		"--fund-p-chain",
		"--force",
		"--receive-recovery-step",
		recoveryStep,
		"--" + constants.SkipUpdateFlag,
	}
	cmd := exec.Command(CLIBinary, args...)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

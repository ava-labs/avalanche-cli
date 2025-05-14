// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commands

import (
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
)

/* #nosec G204 */
func CreateKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		keyName,
		"--"+constants.SkipUpdateFlag,
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
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
	)
	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func CreateKeyForce(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		keyName,
		"--force",
		"--"+constants.SkipUpdateFlag,
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func ListKeys(network string, useNanoAvax bool, subnets string, tokens string) (string, error) {
	args := []string{KeyCmd, "list", "--" + network, "--" + constants.SkipUpdateFlag}
	if useNanoAvax {
		args = append(args, "--use-nano-avax=true")
	}
	if subnets != "" {
		args = append(args, "--subnets", subnets)
	}
	if tokens != "" {
		args = append(args, "--tokens", tokens)
	}
	cmd := exec.Command(CLIBinary, args...)
	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func DeleteKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"delete",
		keyName,
		"--force",
		"--"+constants.SkipUpdateFlag,
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func ExportKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"export",
		keyName,
		"--"+constants.SkipUpdateFlag,
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
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
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func KeyTransferSend(
	args []string,
) (string, error) {
	transferArgs := []string{
		KeyCmd,
		"transfer",
		"--" + constants.SkipUpdateFlag,
	}

	cmd := exec.Command(CLIBinary, append(transferArgs, args...)...)
	outputByte, err := cmd.CombinedOutput()
	return string(outputByte), err
}

/* #nosec G204 */
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

/* #nosec G204 */
func FundKeyOnCchain(senderKeyName, destinationKeyName string, amount int, testFlags utils.TestFlags) (string, error) {
	// Create config
	args := []string{
		"--local",
		"--c-chain-sender",
		"--c-chain-receiver",
		"--" + constants.SkipUpdateFlag,
	}
	return utils.TestCommand(KeyCmd, "transfer", args, utils.GlobalFlags{
		"key":               senderKeyName,
		"destination-key":   destinationKeyName,
		"amount":            amount,
		"local":             true,
		"skip-update-check": true,
	}, testFlags)
}

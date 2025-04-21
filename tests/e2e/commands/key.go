// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commands

import (
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
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
func ListKeys(network string, omitCChain bool, useNanoAvax bool, subnets string) (string, error) {
	args := []string{KeyCmd, "list", "--" + network, "--" + constants.SkipUpdateFlag}
	if omitCChain {
		args = append(args, "--cchain=false")
	}
	if useNanoAvax {
		args = append(args, "--use-nano-avax=true")
	}
	if subnets != "" {
		args = append(args, "--subnets", "c,x,p,"+subnets)
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
func KeyTransferSend(keyName, ledger, destinationAddr, destinationKey, amount, senderChain, senderBlockchainName, receiverChain, receiverBlockchainName, originTransferrerAddress, destinationTransferrerAddress string) *exec.Cmd {
	// Create config
	args := []string{
		KeyCmd,
		"transfer",
		"--local",
		"--" + constants.SkipUpdateFlag,
	}

	if ledger != "" {
		args = append(args, "--ledger", ledger)
	}

	if keyName != "" {
		args = append(args, "--key", keyName)
	}

	if destinationAddr != "" {
		args = append(args, "--destination-addr", destinationAddr)
	}

	if destinationKey != "" {
		args = append(args, "--destination-key", destinationKey)
	}

	if senderChain != "" {
		args = append(args, senderChain)
	}

	if senderBlockchainName != "" {
		args = append(args, senderBlockchainName)
	}

	if receiverChain != "" {
		args = append(args, receiverChain)
	}

	if receiverBlockchainName != "" {
		args = append(args, receiverBlockchainName)
	}

	if amount != "" {
		args = append(args, "--amount", amount)
	}

	if originTransferrerAddress != "" {
		args = append(args, "--origin-transferrer-address", originTransferrerAddress)
	}

	if destinationTransferrerAddress != "" {
		args = append(args, "--destination-transferrer-address", destinationTransferrerAddress)
	}

	cmd := exec.Command(CLIBinary, args...)

	return cmd
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

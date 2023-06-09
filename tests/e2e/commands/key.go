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

/*
  -o, --amount float                 amount to send or receive (AVAX units)
  -f, --force                        avoid transfer confirmation
  -u, --fuji                         transfer between testnet (fuji) addresses
  -h, --help                         help for transfer
  -k, --key string                   key associated to the sender or receiver address
  -i, --ledger uint                  ledger index associated to the sender or receiver address (default 32768)
  -l, --local                        transfer between local network addresses
  -m, --mainnet                      transfer between mainnet addresses
  -g, --receive                      receive the transfer
  -r, --receive-recovery-step uint   receive step to use for multiple step transaction recovery
  -s, --send                         send the transfer
  -a, --target-addr string           receiver address
  -t, --testnet
*/

/* #nosec G204 */
func KeyTransferSend(keyName string, targetAddr string, amount string) (string, error) {
	// Create config
	args := []string{
		KeyCmd,
		"transfer",
		"--local",
		"--key",
		keyName,
		"--send",
		"--target-addr",
		targetAddr,
		"--amount",
		amount,
		"--force",
		"--" + constants.SkipUpdateFlag,
	}
	cmd := exec.Command(CLIBinary, args...)

	out, err := cmd.CombinedOutput()
	return string(out), err
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
		"--force",
		"--receive-recovery-step",
		recoveryStep,
		"--" + constants.SkipUpdateFlag,
	}
	cmd := exec.Command(CLIBinary, args...)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

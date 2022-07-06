package commands

import (
	"os/exec"
)

/* #nosec G204 */
func CreateKey(keyName string) (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"create",
		keyName,
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
	)

	out, err := cmd.Output()
	return string(out), err
}

/* #nosec G204 */
func ListKeys() (string, error) {
	// Create config
	cmd := exec.Command(
		CLIBinary,
		KeyCmd,
		"list",
	)

	out, err := cmd.Output()
	return string(out), err
}

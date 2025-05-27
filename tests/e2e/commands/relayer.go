// / Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
)

/* #nosec G204 */
func StopRelayer() (string, error) {
	return utils.TestCommand(InterchainCMD, "relayer", []string{"stop"}, utils.GlobalFlags{
		"local":             true,
		"skip-update-check": true,
	}, utils.TestFlags{})
}

/* #nosec G204 */
func DeployRelayer(args []string, testFlags utils.TestFlags) (string, error) {
	return utils.TestCommand(InterchainCMD, "relayer", args, utils.GlobalFlags{
		"local":             true,
		"skip-update-check": true,
	}, testFlags)
}

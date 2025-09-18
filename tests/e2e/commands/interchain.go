// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package commands

import (
	"fmt"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/onsi/gomega"
)

const (
	InterchainCMD = "interchain"
)

/* #nosec G204 */
func DeployInterchainTokenTransferrer(args []string) string {
	// Create config
	icctArgs := []string{
		InterchainCMD,
		"tokenTransferrer",
		"deploy",
		"--" + constants.SkipUpdateFlag,
	}

	cmd := exec.Command(CLIBinary, append(icctArgs, args...)...)
	output, err := cmd.CombinedOutput()
	fmt.Println(cmd.String())
	fmt.Println(string(output))
	utils.PrintStdErr(err)
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

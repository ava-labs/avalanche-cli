// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	ICMCmd = "icm"
)

/* #nosec G204 */
func SendICMMessage(network, subnetOne, subnetTwo, message, key string) string {
	// Create config
	cmdArgs := []string{
		ICMCmd,
		"sendMsg",
		network,
		subnetOne,
		subnetTwo,
		message,
		"--key",
		key,
		"--" + constants.SkipUpdateFlag,
	}

	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

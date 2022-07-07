// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"os/exec"

	"github.com/onsi/gomega"
)

/* #nosec G204 */
func CleanNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"clean",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

/* #nosec G204 */
func StartNetwork() string {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"start",
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

/* #nosec G204 */
func StopNetwork() {
	cmd := exec.Command(
		CLIBinary,
		NetworkCmd,
		"stop",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

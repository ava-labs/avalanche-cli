// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"os/exec"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/onsi/gomega"
)

func Create(network string, numNodes int) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"create",
		constants.E2EClusterName,
		"--use-static-ip=false",
		"--latest-avalanchego-version=true",
		"--region local",
		"--num-nodes="+strconv.Itoa(numNodes),
		"--"+network,
		"--node-type=docker",
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func Status() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"status",
		constants.E2EClusterName,
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

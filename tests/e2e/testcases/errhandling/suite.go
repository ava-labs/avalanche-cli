// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package errhandling

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName = "doFailSubnet"
)

/*
		The tests in this suite are meant to trigger some errors so that
	  the UI will try to find errors in the log files.
		However, one or more tests only trigger errors on timeouts,
		which is why it should not be run in normal CI.

		Therefore the tests are `Skip`ed, but can be enabled manually for testing
*/
var _ = ginkgo.Describe("[Error handling]", func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		if err != nil {
			fmt.Println("Clean network error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		// delete custom vm
		utils.DeleteCustomBinary(subnetName)
	})
	ginkgo.It("subnet-evm has error but booted", func() {
		// tip: if you really want to run this, reduce the RequestTimeout
		ginkgo.Skip("run this manually only, times out")
		// this will boot the subnet with a bad genesis:
		// the root gas limit is smaller than the fee config gas limit, should fail
		commands.CreateSubnetEvmConfig(subnetName, utils.SubnetEvmGenesisBadPath)
		out, err := commands.DeploySubnetLocallyWithArgsAndOutput(subnetName, "", "")
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(out).Should(gomega.ContainSubstring("does not match gas limit"))
		fmt.Println(string(out))
	})
})

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
	ginkgo.It("launching faulty VM prints error message", func() {
		commands.CreateSpacesVMConfigWithVersion(subnetName, utils.SpacesVMGenesisPath, "latest")
		commands.DeploySubnetLocallyWithArgsExpectError(subnetName, "v1.8.4", "") // this combination should fail
	})
})

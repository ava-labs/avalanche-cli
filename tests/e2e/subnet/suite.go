// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName  = "e2eSubnetTest"
	genesisPath = "tests/e2e/assets/test_genesis.json"
)

var _ = ginkgo.Describe("[Subnet]", func() {
	ginkgo.It("can create and delete a subnet config", func() {
		commands.CreateSubnetConfig(subnetName, genesisPath)
		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can create and delete a custom vm subnet config", func() {
		customVMPath, err := utils.DownloadCustomVMBin()
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CreateCustomVMSubnetConfig(subnetName, genesisPath, customVMPath)
		commands.DeleteSubnetConfig(subnetName)
		exists, err := utils.SubnetCustomVMExists(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(exists).Should(gomega.BeFalse())
	})
})

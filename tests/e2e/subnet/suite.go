// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
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
})

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const (
	CLIBinary         = "./bin/avalanche"
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
	avalancheGoPath   = "--avalanchego-path"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testLocalNodeName = "e2eSubnetTest-local-node"
)

var _ = ginkgo.Describe("[Etna AddRemove Validator SOV]", func() {
	ginkgo.It("Create Etna Subnet Config", func() {
		commands.CreateEtnaSubnetEvmConfig(
			subnetName,
			ewoqEVMAddress,
		)

	})
})

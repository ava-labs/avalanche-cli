// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	CLIBinary         = "./bin/avalanche"
	keyName           = "ewoq"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
)

var avagoVersion string

var _ = ginkgo.Describe("[Etna Add Validator SOV Local]", func() {
	ginkgo.It("Create Etna Subnet Config", func() {
		_, avagoVersion = commands.CreateEtnaSubnetEvmConfig(
			utils.BlockchainName,
			ewoqEVMAddress,
			commands.PoS,
		)
	})
	ginkgo.It("Can deploy blockchain to localhost and upsize it", func() {
		output := commands.StartNetworkWithParams(map[string]string{
			"version": avagoVersion,
		})
		fmt.Println(output)
		output, err := commands.DeployEtnaBlockchain(
			utils.BlockchainName,
			"",
			nil,
			ewoqPChainAddress,
			false, // convertOnly
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		output, err = commands.AddEtnaSubnetValidatorToCluster(
			"",
			utils.BlockchainName,
			"",
			ewoqPChainAddress,
			1,
			true,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can destroy local node", func() {
		output, err := commands.DestroyLocalNode(utils.TestLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can destroy Etna Local Network", func() {
		_, err := commands.CleanNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("Can remove Etna Subnet Config", func() {
		commands.DeleteSubnetConfig(utils.BlockchainName)
	})
})

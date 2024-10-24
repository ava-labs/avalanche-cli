// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
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
	ginkgo.It("Can create a local node connected to Etna Devnet", func() {
		output, err := commands.CreateLocalEtnaDevnetNode(
			testLocalNodeName,
			7,
			utils.EtnaAvalancheGoBinaryPath,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Deploy Etna Subnet", func() {
		output, err := commands.DeployEtnaSubnetToCluster(
			subnetName,
			testLocalNodeName,
			[]string{
				"http://127.0.0.1:9650",
				"http://127.0.0.1:9652",
				"http://127.0.0.1:9654",
				"http://127.0.0.1:9656",
				"http://127.0.0.1:9658",
			},
			true, // convertOnly
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can make cluster track a subnet", func() {
		output, err := commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can initialize a PoA Manager contract", func() {
		output, err := commands.InitPoaManager(subnetName, testLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9660",
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add second validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9662",
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can remove bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9654",
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can remove non-bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9660",
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can destroy local noce", func() {
		output, err := commands.DestroyLocalNode(testLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})
})

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"regexp"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var blockchainID = ""

const (
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testLocalNodeName = "e2eSubnetTest-local-node"
)

var _ = ginkgo.Describe("[Etna AddRemove Validator SOV PoA]", func() {
	ginkgo.It("Create Etna Subnet Config", func() {
		commands.CreateEtnaSubnetEvmConfig(
			subnetName,
			ewoqEVMAddress,
			commands.PoA,
		)
	})
	ginkgo.It("Can create a local node connected to Etna Devnet", func() {
		output, err := commands.CreateLocalEtnaDevnetNode(
			testLocalNodeName,
			7,
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
			ewoqPChainAddress,
			true, // convertOnly
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can make cluster track a subnet", func() {
		output, err := commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		// parse blockchainID from output
		re := regexp.MustCompile(`Waiting for rpc http://.*?/bc/([A-Za-z0-9]+)/rpc`)
		// Find the first match
		match := re.FindStringSubmatch(output)
		gomega.Expect(match).ToNot(gomega.BeEmpty())
		if len(match) > 1 {
			// The first submatch will contain the chain ID
			blockchainID = match[1]
		}
		gomega.Expect(blockchainID).Should(gomega.Not(gomega.BeEmpty()))
		ginkgo.GinkgoWriter.Printf("Blockchain ID: %s\n", blockchainID)
	})

	ginkgo.It("Can initialize a PoA Manager contract", func() {
		output, err := commands.InitValidatorManager(subnetName,
			testLocalNodeName,
			"http://127.0.0.1:9650",
			blockchainID,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9660",
			ewoqPChainAddress,
			1,
			false, // use existing avago running
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add second validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9662",
			ewoqPChainAddress,
			1,
			false, // use existing avago running
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can get status of the cluster", func() {
		output, err := commands.GetLocalClusterStatus(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		// make sure we can find string with "http://127.0.0.1:9660" and "L1:Validating" string in the output
		gomega.Expect(output).To(gomega.MatchRegexp(`http://127\.0\.0\.1:9652.*Validating`), "expect to have L1 validating")
		// make sure we can do the same for "http://127.0.0.1:9662"
		gomega.Expect(output).To(gomega.MatchRegexp(`http://127\.0\.0\.1:9654.*Validating`), "expect to have L1 validating")
	})

	ginkgo.It("Can remove bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9654",
			keyName,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can remove non-bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			testLocalNodeName,
			subnetName,
			"http://127.0.0.1:9660",
			keyName,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})
	ginkgo.It("Can destroy local node", func() {
		output, err := commands.DestroyLocalNode(testLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})
})

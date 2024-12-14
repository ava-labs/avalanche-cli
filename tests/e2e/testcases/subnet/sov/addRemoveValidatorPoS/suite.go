// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

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

var (
	blockchainID     string
	localClusterUris []string
)

var _ = ginkgo.Describe("[Etna AddRemove Validator SOV PoS]", func() {
	ginkgo.It("Create Etna Subnet Config", func() {
		commands.CreateEtnaSubnetEvmConfig(
			utils.SubnetName,
			ewoqEVMAddress,
			commands.PoS,
		)
	})

	ginkgo.It("Can create an Etna Local Network", func() {
		output := commands.StartNetwork()
		fmt.Println(output)
	})

	ginkgo.It("Can create a local node connected to Etna Local Network", func() {
		output, err := commands.CreateLocalEtnaNode(
			utils.TestLocalNodeName,
			7,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		localClusterUris, err = utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(7))
	})

	ginkgo.It("Deploy Etna Subnet", func() {
		output, err := commands.DeployEtnaSubnetToCluster(
			utils.SubnetName,
			utils.TestLocalNodeName,
			[]string{
				localClusterUris[0],
				localClusterUris[1],
				localClusterUris[2],
				localClusterUris[3],
				localClusterUris[4],
			},
			ewoqPChainAddress,
			true, // convertOnly
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can make cluster track a subnet", func() {
		output, err := commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.SubnetName)
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

	ginkgo.It("Can initialize a PoS Manager contract", func() {
		output, err := commands.InitValidatorManager(utils.SubnetName,
			utils.TestLocalNodeName,
			localClusterUris[0],
			blockchainID,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			utils.TestLocalNodeName,
			utils.SubnetName,
			localClusterUris[5],
			ewoqPChainAddress,
			1,
			100,
			false, // use existing
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can add second validator", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			utils.TestLocalNodeName,
			utils.SubnetName,
			localClusterUris[6],
			ewoqPChainAddress,
			1,
			100,
			false, // use existing
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can get status of thecluster", func() {
		output, err := commands.GetLocalClusterStatus(utils.TestLocalNodeName, utils.SubnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		// make sure we can find string with "http://127.0.0.1:port" and "L1:Validating" string in the output
		parsedURL, err := url.Parse(localClusterUris[1])
		gomega.Expect(err).Should(gomega.BeNil())
		port := parsedURL.Port()
		gomega.Expect(port).Should(gomega.Not(gomega.BeEmpty()))
		regexp := fmt.Sprintf(`http://127\.0\.0\.1:%s.*Validating`, port)
		gomega.Expect(output).To(gomega.MatchRegexp(regexp), fmt.Sprintf("expect to have L1 validated by port %s", port))
		parsedURL, err = url.Parse(localClusterUris[2])
		gomega.Expect(err).Should(gomega.BeNil())
		port = parsedURL.Port()
		gomega.Expect(port).Should(gomega.Not(gomega.BeEmpty()))
		regexp = fmt.Sprintf(`http://127\.0\.0\.1:%s.*Validating`, port)
		gomega.Expect(output).To(gomega.MatchRegexp(regexp), fmt.Sprintf("expect to have L1 validated by port %s", port))
	})

	ginkgo.It("Can sleep for min stake duration", func() {
		time.Sleep(3 * time.Minute)
	})

	ginkgo.It("Can remove bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			utils.TestLocalNodeName,
			utils.SubnetName,
			localClusterUris[2],
			keyName,
			0,
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
		commands.CleanNetwork()
	})

	ginkgo.It("Can remote Etna Subnet Config", func() {
		commands.DeleteSubnetConfig(utils.SubnetName)
	})
})

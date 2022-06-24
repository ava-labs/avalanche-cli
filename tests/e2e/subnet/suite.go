package subnet

import (
	"time"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const subnetName = "e2eSubnetTest"

var _ = ginkgo.Describe("[Subnet]", func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can create and delete a subnet config", func() {
		genesis := "tests/e2e/genesis/test_genesis.json"

		commands.CreateSubnetConfig(subnetName, genesis)
		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy a subnet", func() {
		genesis := "tests/e2e/genesis/test_genesis.json"

		commands.CreateSubnetConfig(subnetName, genesis)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpc, err := utils.ParseRPCFromDeployOutput(deployOutput)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		// Subnet doesn't seem to accept JSON requests from hardhat right away
		// Test fails without this
		time.Sleep(60 * time.Second)

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})
})

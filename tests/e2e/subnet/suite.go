package subnet

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Subnet]", func() {
	ginkgo.It("can create and delete a subnet config", func() {
		subnetName := "e2eSubnetTest"
		genesis := "tests/e2e/genesis/test_genesis.json"

		commands.CreateSubnetConfig(subnetName, genesis)
		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy a subnet", func() {
		subnetName := "e2eSubnetTest"
		genesis := "tests/e2e/genesis/test_genesis.json"

		commands.CreateSubnetConfig(subnetName, genesis)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpc, err := utils.ParseRPCFromOutput(deployOutput)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println("Found rpc", rpc)

		commands.CleanNetwork()

		commands.DeleteSubnetConfig(subnetName)
	})
})

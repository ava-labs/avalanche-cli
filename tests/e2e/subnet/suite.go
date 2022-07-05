package subnet

import (
	"fmt"

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
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can create and delete a subnet config", func() {
		commands.CreateSubnetConfig(subnetName, genesisPath)
		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy a subnet", func() {
		commands.CreateSubnetConfig(subnetName, genesisPath)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpc, err := utils.ParseRPCFromDeployOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})
})

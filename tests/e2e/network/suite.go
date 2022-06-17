package network

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const subnetName = "e2eSubnetTest"

var _ = ginkgo.Describe("[Network]", func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can stop and restart a deployed subnet", func() {
		genesis := "tests/e2e/genesis/test_genesis.json"

		commands.CreateSubnetConfig(subnetName, genesis)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		_, err := utils.ParseRPCFromDeployOutput(deployOutput)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.StopNetwork()
		restartOutput := commands.StartNetwork()
		rpc, err := utils.ParseRPCFromRestartOutput(restartOutput)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(rpc)

		commands.DeleteSubnetConfig(subnetName)
	})
})

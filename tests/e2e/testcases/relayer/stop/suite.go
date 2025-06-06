package stop

import (
	"github.com/ava-labs/avalanche-cli/cmd"
	"github.com/ava-labs/avalanche-cli/pkg/interchain/relayer"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName = "ewoq"
	keyName     = "e2eKey"
	subnetName  = "testSubnet"
	cChain      = "cchain"
)

var _ = ginkgo.Describe("[Relayer] stop", func() {
	ginkgo.BeforeEach(func() {
		_, err := commands.CreateKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.StartNetwork()
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
		commands.DeploySubnetLocallyNonSOV(subnetName)
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		_, err := commands.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteCustomBinary(subnetName)
	})

	ginkgo.Context("With valid input", func() {
		ginkgo.It("should stop the relayer", func() {
			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			_, err = commands.DeployRelayer(
				[]string{
					"deploy",
					"--cchain",
				},
				utils.TestFlags{
					"key":           ewoqKeyName,
					"blockchains":   subnetName,
					"amount":        10000,
					"cchain-amount": 10000,
					"log-level":     "info",
				})
			gomega.Expect(err).Should(gomega.BeNil())

			app := utils.GetApp()
			configPath := app.GetLocalRelayerRunPath(models.Local)
			isUp, _, _, err := relayer.RelayerIsUp(configPath)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(isUp).Should(gomega.BeTrue())

			// Stop the relayer
			output, err := commands.StopRelayer()
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Local AWM Relayer successfully stopped for Local Network"))

			isUp, _, _, err = relayer.RelayerIsUp(configPath)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(isUp).Should(gomega.BeFalse())
		})
	})

	ginkgo.Context("With invalid input", func() {
		ginkgo.It("should fail to start the relayer when there is no relayer config", func() {
			// Start the relayer
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", []string{
				"start",
			}, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, utils.TestFlags{})

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("there is no relayer configuration available"))
		})

		ginkgo.It("should fail to start the relayer when it is already running", func() {
			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			_, err = commands.DeployRelayer(
				[]string{
					"deploy",
					"--cchain",
				},
				utils.TestFlags{
					"key":           ewoqKeyName,
					"blockchains":   subnetName,
					"amount":        10000,
					"cchain-amount": 10000,
					"log-level":     "info",
				})
			gomega.Expect(err).Should(gomega.BeNil())

			// Start the relayer
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", []string{
				"start",
			}, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, utils.TestFlags{})

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("local AWM relayer is already running for Local Network"))
		})
	})
})

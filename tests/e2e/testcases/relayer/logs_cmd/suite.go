package logs

import (
	"github.com/ava-labs/avalanche-cli/cmd"
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
		ginkgo.It("should display logs", func() {
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

			// Display relayer logs
			logsArgs := []string{
				"logs",
			}
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", logsArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, utils.TestFlags{})
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Initializing icm-relayer"))
		})

		ginkgo.It("should display raw logs", func() {
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

			// Display relayer logs
			logsArgs := []string{
				"logs",
				"--raw",
			}
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", logsArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, utils.TestFlags{})
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Initializing icm-relayer"))
		})

		ginkgo.It("should display first logs", func() {
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

			// Display relayer logs
			logsFlags := utils.TestFlags{
				"first": 3,
			}
			logsArgs := []string{
				"logs",
				"--raw",
			}
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", logsArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, logsFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Initializing icm-relayer"))
			gomega.Expect(output).Should(gomega.ContainSubstring("Initializing destination clients"))
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("Initializing source clients"))
		})

		ginkgo.It("should display last logs", func() {
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

			// Display relayer logs
			logsFlags := utils.TestFlags{
				"last": 3,
			}
			logsArgs := []string{
				"logs",
				"--raw",
			}
			output, err := utils.TestCommand(cmd.InterchainCmd, "relayer", logsArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, logsFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("Initializing icm-relayer"))
			gomega.Expect(output).Should(gomega.ContainSubstring("Listener initialized. Listening for messages to relay."))
		})
	})
})

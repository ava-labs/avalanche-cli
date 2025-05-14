package deploy

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName = "ewoq"
	keyName     = "e2eKey"
	subnetName  = "testSubnet"
	subnet2Name = "testSubnet2"
	cChain      = "cchain"
	message     = "Hello World"
)

var _ = ginkgo.Describe("[Relayer] deploy", func() {
	ginkgo.BeforeEach(func() {
		_, err := commands.CreateKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.StartNetwork()
		commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
		commands.DeploySubnetLocallyNonSOV(subnetName)
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteCustomBinary(subnetName)

		err = utils.DeleteConfigs(subnet2Name)
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteCustomBinary(subnet2Name)

		_, err = commands.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.FContext("With valid input", func() {
		ginkgo.It("should deploy the relayer between c-chain and subnet in both directions", func() {
			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":           keyName,
				"blockchains":   subnetName,
				"amount":        10000,
				"cchain-amount": 10000,
				"log-level":     "info",
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Executing Relayer"))

			// Send message from c-chain to subnet
			_, err = commands.SendICMMessage(
				[]string{
					"cchain",
					subnetName,
					"hello world",
				},
				utils.TestFlags{
					"key": ewoqKeyName,
				})
			gomega.Expect(err).Should(gomega.BeNil())

			// Send message from subnet to c-chain
			_, err = commands.SendICMMessage(
				[]string{
					subnetName,
					"cchain",
					"hello world",
				},
				utils.TestFlags{
					"key": ewoqKeyName,
				})
			gomega.Expect(err).Should(gomega.BeNil())
		})

		ginkgo.It("should deploy the relayer between subnet and subnet in both directions", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnet2Name, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnet2Name)

			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			_, err = commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnet2Name,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":           keyName,
				"blockchains":   fmt.Sprintf("%s,%s", subnetName, subnet2Name),
				"amount":        10000,
				"cchain-amount": 10000,
				"log-level":     "info",
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Executing Relayer"))

			// Send message from c-chain to subnet
			_, err = commands.SendICMMessage(
				[]string{
					subnetName,
					subnet2Name,
					"hello world",
				},
				utils.TestFlags{
					"key": ewoqKeyName,
				})
			gomega.Expect(err).Should(gomega.BeNil())

			// Send message from subnet to c-chain
			_, err = commands.SendICMMessage(
				[]string{
					subnet2Name,
					subnetName,
					"hello world",
				},
				utils.TestFlags{
					"key": ewoqKeyName,
				})
			gomega.Expect(err).Should(gomega.BeNil())
		})
	})
})

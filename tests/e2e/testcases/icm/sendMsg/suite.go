package sendmsg

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName = "ewoq"
	subnetName  = "testSubnet"
	cChain      = "cchain"
)

var _ = ginkgo.Describe("[ICM] sendMsg", func() {
	ginkgo.FContext("with valid input", func() {
		ginkgo.BeforeEach(func() {
			commands.StartNetwork()
		})

		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
			err := utils.DeleteConfigs(subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			utils.DeleteCustomBinary(subnetName)
		})

		ginkgo.It("should send a message from c-chain to subnet", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}

			// Deploy ICM
			icmDeployFlags := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			relayerDeployFlags := utils.TestFlags{
				"key":           ewoqKeyName,
				"blockchains":   subnetName,
				"amount":        10000,
				"cchain-amount": 10000,
				"log-level":     "info",
			}
			relayerDeployArgs := []string{
				"deploy",
				"--cchain",
			}

			_, err = utils.TestCommand(utils.InterchainCMD, "relayer", relayerDeployArgs, globalFlags, relayerDeployFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			// Send a message
			message := "Hello World"
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				cChain,
				subnetName,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Delivering message \"%s\" from source blockchain \"%s\"", message, cChain)))
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Waiting for message to be delivered to destination blockchain \"%s\"", subnetName)))
			gomega.Expect(output).Should(gomega.ContainSubstring("Message successfully Teleported!"))

			commands.StopRelayer()
		})
	})
})

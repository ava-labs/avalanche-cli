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
	subnet2Name = "testSubnet2"
	cChain      = "cchain"
	message     = "Hello World"
)

var _ = ginkgo.Describe("[ICM] sendMsg", func() {
	ginkgo.BeforeEach(func() {
		commands.StartNetwork()
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteCustomBinary(subnetName)

		err = utils.DeleteConfigs(subnet2Name)
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteCustomBinary(subnet2Name)
	})

	ginkgo.Context("with valid input", func() {
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

		ginkgo.It("should send a message from subnet to cchain", func() {
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
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				subnetName,
				cChain,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Delivering message \"%s\" from source blockchain \"%s\"", message, subnetName)))
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Waiting for message to be delivered to destination blockchain \"%s\"", cChain)))
			gomega.Expect(output).Should(gomega.ContainSubstring("Message successfully Teleported!"))

			commands.StopRelayer()
		})

		ginkgo.It("should send a message from subnet to subnet", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			commands.CreateSubnetEvmConfigNonSOV(subnet2Name, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnet2Name)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}

			// Deploy ICM to subnet1
			icmDeployFlags1 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags1)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy ICM to subnet2
			icmDeployFlags2 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnet2Name,
			}

			_, err = utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags2)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			relayerDeployFlags := utils.TestFlags{
				"key":           ewoqKeyName,
				"blockchains":   fmt.Sprintf("%s,%s", subnetName, subnet2Name),
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
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				subnet2Name,
				subnetName,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Delivering message \"%s\" from source blockchain \"%s\"", message, subnet2Name)))
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Waiting for message to be delivered to destination blockchain \"%s\"", subnetName)))
			gomega.Expect(output).Should(gomega.ContainSubstring("Message successfully Teleported!"))

			commands.StopRelayer()
		})

		ginkgo.It("should send a message from subnet to subnet", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			commands.CreateSubnetEvmConfigNonSOV(subnet2Name, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnet2Name)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}

			// Deploy ICM to subnet1
			icmDeployFlags1 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags1)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy ICM to subnet2
			icmDeployFlags2 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnet2Name,
			}

			_, err = utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags2)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			relayerDeployFlags := utils.TestFlags{
				"key":           ewoqKeyName,
				"blockchains":   fmt.Sprintf("%s,%s", subnetName, subnet2Name),
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
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				subnet2Name,
				subnetName,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Delivering message \"%s\" from source blockchain \"%s\"", message, subnet2Name)))
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Waiting for message to be delivered to destination blockchain \"%s\"", subnetName)))
			gomega.Expect(output).Should(gomega.ContainSubstring("Message successfully Teleported!"))

			commands.StopRelayer()
		})

		ginkgo.It("should send a message from subnet to subnet with set rpc endpoints", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			commands.CreateSubnetEvmConfigNonSOV(subnet2Name, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnet2Name)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}

			// Deploy ICM to subnet1
			icmDeployFlags1 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags1)
			gomega.Expect(err).Should(gomega.BeNil())

			rpcs1, err := utils.ParseRPCsFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy ICM to subnet2
			icmDeployFlags2 := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnet2Name,
			}

			output, err = utils.TestCommand(utils.ICMCmd, "deploy", []string{}, globalFlags, icmDeployFlags2)
			gomega.Expect(err).Should(gomega.BeNil())

			rpcs2, err := utils.ParseRPCsFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy the relayer
			relayerDeployFlags := utils.TestFlags{
				"key":           ewoqKeyName,
				"blockchains":   fmt.Sprintf("%s,%s", subnetName, subnet2Name),
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
			sendMsgFlags := utils.TestFlags{
				"key":        ewoqKeyName,
				"source-rpc": rpcs2[0],
				"dest-rpc":   rpcs1[0],
			}

			sendMessageArgs := []string{
				subnet2Name,
				subnetName,
				message,
			}

			output, err = utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Delivering message \"%s\" from source blockchain \"%s\"", message, subnet2Name)))
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Waiting for message to be delivered to destination blockchain \"%s\"", subnetName)))
			gomega.Expect(output).Should(gomega.ContainSubstring("Message successfully Teleported!"))

			commands.StopRelayer()
		})
	})

	ginkgo.Context("with invalid input", func() {
		ginkgo.It("should fail to send a message with invalid source blockchain", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				subnetName,
				cChain,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).ShouldNot(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("subnet \"%s\" does not exist", subnetName)))
		})

		ginkgo.It("should fail to send a message with invalid destination blockchain", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			sendMsgFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			sendMessageArgs := []string{
				cChain,
				subnetName,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).ShouldNot(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("subnet \"%s\" does not exist", subnetName)))
		})

		ginkgo.It("should fail to send a message with invalid source rpc endpoint", func() {
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
			sendMsgFlags := utils.TestFlags{
				"key":        ewoqKeyName,
				"source-rpc": "http://127.0.0.1:61171/ext/bc/invalid-subnet/rpc",
			}

			sendMessageArgs := []string{
				subnetName,
				cChain,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).ShouldNot(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Post \"http://127.0.0.1:61171/ext/bc/invalid-subnet/rpc\": dial tcp 127.0.0.1:61171: connect: connection refused"))
		})

		ginkgo.It("should fail to send a message with invalid destination rpc endpoint", func() {
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
			sendMsgFlags := utils.TestFlags{
				"key":      ewoqKeyName,
				"dest-rpc": "http://127.0.0.1:61171/ext/bc/invalid-subnet/rpc",
			}

			sendMessageArgs := []string{
				subnetName,
				cChain,
				message,
			}

			output, err := utils.TestCommand(utils.ICMCmd, "sendMsg", sendMessageArgs, globalFlags, sendMsgFlags)
			gomega.Expect(err).ShouldNot(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("Post \"http://127.0.0.1:61171/ext/bc/invalid-subnet/rpc\": dial tcp 127.0.0.1:61171: connect: connection refused"))
		})
	})
})

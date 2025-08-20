package deploy

import (
	"github.com/ava-labs/avalanche-cli/cmd"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName      = "ewoq"
	subnetName       = "testSubnet"
	cChainRPCUrl     = "http://127.0.0.1:9650/ext/bc/C/rpc"
	cChainSubnetName = "c-chain"
)

var globalFlags = utils.GlobalFlags{
	"local":             true,
	"skip-update-check": true,
}

var evmClient evm.Client

var _ = ginkgo.Describe("[Validator Manager Deploy]", func() {
	ginkgo.Context("with valid input", func() {
		ginkgo.BeforeEach(func() {
			commands.StartNetwork()
			var err error
			evmClient, err = evm.GetClient(cChainRPCUrl)
			gomega.Expect(err).Should(gomega.BeNil())
		})
		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
			err := utils.DeleteConfigs(subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
		})
		ginkgo.It("should deploy PoA validator manager contract into c-chain", func() {
			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--poa",
				"--deploy-proxy",
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err := utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Admin Address: "))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err := utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err := evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyAdminContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
		})
		ginkgo.It("should deploy PoS validator manager contract into c-chain", func() {
			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--pos",
				"--deploy-proxy",
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err := utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Admin Address: "))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err := utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err := evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyAdminContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
		})
		ginkgo.It("should deploy arbitrary validator manager contract into c-chain", func() {
			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--validator-manager-path",
				utils.ValidatorManagerContractPath,
				"--deploy-proxy",
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err := utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Admin Address: "))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err := utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err := evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyAdminContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
		})
		ginkgo.It("should deploy PoA validator manager contract into c-chain with proxy update", func() {
			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--poa",
				"--deploy-proxy",
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err := utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Admin Address: "))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err := utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err := evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyAdminContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())

			commandArguments = []string{
				"validatorManager",
				"--poa",
				"--deploy-proxy=false",
				"--proxy",
				proxyContract,
				"--proxy-admin",
				proxyAdminContract,
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err = utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Updating proxy"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy successfully configured"))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err = utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err = evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			gomega.Expect(proxyContract).Should(gomega.Equal(""))
			gomega.Expect(proxyAdminContract).Should(gomega.Equal(""))
		})
		ginkgo.It("should deploy PoA validator manager contract into subnet", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			output := commands.DeploySubnetLocallyNonSOV(subnetName)
			rpcUrls, err := utils.ParseRPCsFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			evmClient, err := evm.GetClient(rpcUrls[0])
			gomega.Expect(err).Should(gomega.BeNil())

			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--poa",
				"--deploy-proxy",
				"--blockchain",
				subnetName,
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}

			output, err = utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Validator Manager Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Address: "))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("Proxy Admin Address: "))

			// Check that contracts are actually deployed
			validatorManagerContract, proxyContract, proxyAdminContract, err := utils.ParseValidatorManagerAddressesFromOutput(output)
			gomega.Expect(err).Should(gomega.BeNil())
			deployed, err := evmClient.ContractAlreadyDeployed(validatorManagerContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
			deployed, err = evmClient.ContractAlreadyDeployed(proxyAdminContract)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(deployed).Should(gomega.BeTrue())
		})
	})
	ginkgo.Context("with invalid input", func() {
		ginkgo.BeforeEach(func() {
			commands.StartNetwork()
		})

		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
		})
		ginkgo.It("should fail with invalid mutually exclusive fields (proxy flags)", func() {
			testFlags := utils.TestFlags{}
			commandArguments := []string{
				"validatorManager",
				"--poa",
				"--deploy-proxy",
				"--proxy",
				"fake_proxy",
				"--proxy-admin",
				"fake_proxy_admin",
				"--c-chain",
				"--genesis-key",
				"--proxy-owner-genesis-key",
			}
			output, err := utils.TestCommand(cmd.ContractCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("can't ask to deploy a proxy while providing either proxy admin or proxy address as input"))
		})
	})
})

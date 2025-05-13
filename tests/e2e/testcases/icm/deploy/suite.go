package deploy

import (
	"os"
	"path"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName = "ewoq"
	subnetName  = "testSubnet"
)

var _ = ginkgo.Describe("[ICM] deploy", func() {
	ginkgo.Context("with valid input", func() {
		ginkgo.BeforeEach(func() {
			commands.StartNetwork()
		})

		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
			err := utils.DeleteConfigs(subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			utils.DeleteCustomBinary(subnetName)
		})
		ginkgo.It("should deploy ICM contracts into c-chain", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM contracts into subnet (including c-chain)", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			}
			commandArguments := []string{}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to " + subnetName))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to " + subnetName))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to c-chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to c-chain"))
		})

		ginkgo.It("should deploy ICM messenger into C-Chain", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":             ewoqKeyName,
				"deploy-registry": "false",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM registry into C-Chain", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":              ewoqKeyName,
				"deploy-messenger": "false",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM messenger into subnet (including c-chain)", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":             ewoqKeyName,
				"blockchain":      subnetName,
				"deploy-registry": "false",
			}
			commandArguments := []string{}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to " + subnetName))
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Registry successfully deployed to " + subnetName))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to c-chain"))
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Registry successfully deployed to c-chain"))
		})

		ginkgo.It("should deploy ICM registry into subnet (including c-chain)", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":              ewoqKeyName,
				"blockchain":       subnetName,
				"deploy-messenger": "false",
			}
			commandArguments := []string{}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Messenger successfully deployed to " + subnetName))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to " + subnetName))
			gomega.Expect(output).
				ShouldNot(gomega.ContainSubstring("ICM Messenger successfully deployed to c-chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to c-chain"))
		})

		ginkgo.It("should not re-deploy ICM contracts if already deployed", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}
			commandArguments := []string{
				"--c-chain",
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Messenger has already been deployed to C-Chain"))
		})

		ginkgo.It("should force deploy ICM registry with messenger already deployed", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key": ewoqKeyName,
			}

			_, err := utils.TestCommand(
				utils.ICMCmd,
				"deploy",
				[]string{
					"--c-chain",
				},
				globalFlags,
				testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(
				utils.ICMCmd,
				"deploy",
				[]string{
					"--c-chain",
					"--force-registry-deploy",
				},
				globalFlags,
				testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Messenger has already been deployed to C-Chain"))
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM registry with messenger not deployed", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"deploy-messenger": "false",
				"key":              ewoqKeyName,
			}
			commandArguments := []string{
				"--c-chain",
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("ICM Messenger has already been deployed to C-Chain"))
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should force deploy ICM registry with messenger not deployed", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"deploy-messenger": "false",
				"key":              ewoqKeyName,
			}
			commandArguments := []string{
				"--c-chain",
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", append(commandArguments, "--force-registry-deploy"), globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("ICM Messenger has already been deployed to C-Chain"))
			gomega.Expect(output).ShouldNot(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM messenger and force deploy registry", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"deploy-messenger": "false",
				"key":              ewoqKeyName,
			}
			commandArguments := []string{
				"--c-chain",
			}

			_, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", append(commandArguments, "--force-registry-deploy"), globalFlags, utils.TestFlags{
				"key": ewoqKeyName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})

		ginkgo.It("should deploy ICM contracts from paths", func() {
			td := interchain.ICMDeployer{}
			contractsDirPath := path.Join(utils.GetBaseDir(), constants.AvalancheCliBinDir, constants.ICMContractsInstallDir)
			version := "v1.0.0"
			// Download contracts
			err := td.DownloadAssets(
				contractsDirPath,
				version,
			)
			gomega.Expect(err).Should(gomega.BeNil())

			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":                             ewoqKeyName,
				"messenger-contract-address-path": filepath.Join(contractsDirPath, version, "TeleporterMessenger_Contract_Address_v1.0.0.txt"),
				"messenger-deployer-address-path": filepath.Join(contractsDirPath, version, "TeleporterMessenger_Deployer_Address_v1.0.0.txt"),
				"messenger-deployer-tx-path":      filepath.Join(contractsDirPath, version, "TeleporterMessenger_Deployment_Transaction_v1.0.0.txt"),
				"registry-bytecode-path":          filepath.Join(contractsDirPath, version, "TeleporterRegistry_Bytecode_v1.0.0.txt"),
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))

			_ = os.RemoveAll(filepath.Join(contractsDirPath, version))
		})

		ginkgo.It("should deploy ICM contracts with version", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":     ewoqKeyName,
				"version": "v1.0.0",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Messenger successfully deployed to C-Chain"))
			gomega.Expect(output).
				Should(gomega.ContainSubstring("ICM Registry successfully deployed to C-Chain"))
		})
	})
	ginkgo.Context("with invalid input", func() {
		ginkgo.BeforeEach(func() {
			commands.StartNetwork()
		})

		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
		})
		ginkgo.It("should fail with invalid mutually exclusive fields (network flags)", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"blockchain":    "test",
				"blockchain-id": "test",
			}
			commandArguments := []string{}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("are mutually exclusive flags"))
		})

		ginkgo.It("should faile with both deploy messenger and deploy registry set to false", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"deploy-messenger": "false",
				"deploy-registry":  "false",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("you should set at least one of --deploy-messenger/--deploy-registry to true"))
		})

		ginkgo.It("should fail with one of the contract paths set", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":                             ewoqKeyName,
				"messenger-contract-address-path": "./test/path",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("if setting any ICM asset path, you must set all ICM asset paths"))
		})

		ginkgo.It("should fail with invalid version", func() {
			globalFlags := utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}
			testFlags := utils.TestFlags{
				"key":     ewoqKeyName,
				"version": "v0.122.5321",
			}
			commandArguments := []string{
				"--c-chain",
			}

			output, err := utils.TestCommand(utils.ICMCmd, "deploy", commandArguments, globalFlags, testFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("failed downloading"))
		})
	})
})

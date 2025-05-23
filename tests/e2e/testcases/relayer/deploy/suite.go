package deploy

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/utils/units"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	ewoqKeyName = "ewoq"
	keyName     = "e2eKey"
	key2Name    = "e2eKey2"
	subnetName  = "testSubnet"
	subnet2Name = "testSubnet2"
	cChain      = "cchain"
	message     = "Hello World"
)

var _ = ginkgo.Describe("[Relayer] deploy", func() {
	ginkgo.BeforeEach(func() {
		_, err := commands.CreateKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.CreateKey(key2Name)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.StartNetwork()
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		_, err := commands.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.DeleteKey(key2Name)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.Context("With valid input", func() {
		ginkgo.Context("With non SOV subnet", func() {
			ginkgo.BeforeEach(func() {
				commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
				commands.DeploySubnetLocallyNonSOV(subnetName)
			})

			ginkgo.AfterEach(func() {
				err := utils.DeleteConfigs(subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				utils.DeleteCustomBinary(subnetName)

				err = utils.DeleteConfigs(subnet2Name)
				gomega.Expect(err).Should(gomega.BeNil())
				utils.DeleteCustomBinary(subnet2Name)
			})

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
						cChain,
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
						cChain,
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
					"key":         keyName,
					"blockchains": fmt.Sprintf("%s,%s", subnetName, subnet2Name),
					"amount":      10000,
					"log-level":   "info",
				}

				deployArgs := []string{
					"deploy",
				}

				output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
					"local":             true,
					"skip-update-check": true,
				}, deployFlags)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(output).Should(gomega.ContainSubstring("Executing Relayer"))

				// Send message from subnet to subnet2
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

				// Send message from subnet2 to subnet
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

			ginkgo.It("should deploy the relayer between subnet, subnet and c-chain in all directions", func() {
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

				// Send message from subnet to c-chain
				_, err = commands.SendICMMessage(
					[]string{
						subnetName,
						cChain,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from subnet2 to c-chain
				_, err = commands.SendICMMessage(
					[]string{
						subnet2Name,
						cChain,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from c-chain to subnet
				_, err = commands.SendICMMessage(
					[]string{
						cChain,
						subnetName,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from c-chain to subnet2
				_, err = commands.SendICMMessage(
					[]string{
						cChain,
						subnet2Name,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from subnet to subnet2
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

				// Send message from subnet2 to subnet
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

			ginkgo.It("should deploy the relayer on c-chain and subnet with funding it from another key", func() {
				// Fund key on subnet
				commandArguments := []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					keyName,
					"--sender-blockchain",
					subnetName,
					"--receiver-blockchain",
					subnetName,
					"--amount",
					"100",
				}
				_, err := commands.KeyTransferSend(commandArguments)
				gomega.Expect(err).Should(gomega.BeNil())

				// Fund key on c-chain
				commandArguments = []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					keyName,
					"--c-chain-sender",
					"--c-chain-receiver",
					"--amount",
					"100",
				}
				_, err = commands.KeyTransferSend(commandArguments)
				gomega.Expect(err).Should(gomega.BeNil())

				// Deploy ICM contracts
				_, err = commands.DeployICMContracts([]string{}, utils.TestFlags{
					"key":        ewoqKeyName,
					"blockchain": subnetName,
				})
				gomega.Expect(err).Should(gomega.BeNil())

				// Deploy relayer
				deployFlags := utils.TestFlags{
					"key":                    key2Name,
					"cchain-amount":          90,
					"blockchains":            subnetName,
					"amount":                 50,
					"log-level":              "info",
					"cchain-funding-key":     keyName,
					"blockchain-funding-key": keyName,
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

				subnetFee, err := utils.GetKeyTransferFee(output, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())

				cchainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				fmt.Println("output", output)

				// Key2 balance on c-chain
				output, err = commands.ListKeys("local", true, "", "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, key2BalanceCchain, err := utils.ParseAddrBalanceFromKeyListOutput(output, key2Name, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(key2BalanceCchain).Should(gomega.Equal(90 * units.Avax))

				// Key2 balance on subnet
				output, err = commands.ListKeys("local", true, "c,"+subnetName, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, key2BalanceSubnet, err := utils.ParseAddrBalanceFromKeyListOutput(output, key2Name, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(key2BalanceSubnet).Should(gomega.Equal(50 * units.Avax))

				// Key balance on c-chain
				output, err = commands.ListKeys("local", true, "", "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalanceCchain, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(keyBalanceCchain).Should(gomega.Equal(10*units.Avax - cchainFee))

				// Key balance on c-chain
				output, err = commands.ListKeys("local", true, "c,"+subnetName, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalanceSubnet, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(keyBalanceSubnet).Should(gomega.Equal(50*units.Avax - subnetFee))
			})
		})

		ginkgo.Context("With SOV subnet", func() {
			ginkgo.BeforeEach(func() {
				commands.CreateSubnetEvmConfigSOV(subnet2Name, utils.SubnetEvmGenesisPoaPath)
				commands.DeploySubnetLocallyNonSOV(subnet2Name)
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

			ginkgo.It("should deploy the relayer between c-chain and subnet (sov) in both directions", func() {
				// Deploy ICM contracts
				_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
					"key":        ewoqKeyName,
					"blockchain": subnet2Name,
				})
				gomega.Expect(err).Should(gomega.BeNil())

				// Deploy relayer
				deployFlags := utils.TestFlags{
					"key":           keyName,
					"blockchains":   subnet2Name,
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
						cChain,
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
						cChain,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())
			})

			ginkgo.It("should deploy the relayer between subnet (sov) and subnet (non-sov) in both directions", func() {
				commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
				commands.DeploySubnetLocallyNonSOV(subnetName)

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
					"key":         keyName,
					"blockchains": fmt.Sprintf("%s,%s", subnetName, subnet2Name),
					"amount":      10000,
					"log-level":   "info",
				}

				deployArgs := []string{
					"deploy",
				}

				output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
					"local":             true,
					"skip-update-check": true,
				}, deployFlags)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(output).Should(gomega.ContainSubstring("Executing Relayer"))

				// Send message from subnet to subnet2
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

				// Send message from subnet2 to subnet
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

			ginkgo.It("should deploy the relayer between subnet (sov), subnet (sov) and c-chain in all directions", func() {
				commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)
				commands.DeploySubnetLocallyNonSOV(subnetName)

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

				// Send message from subnet to c-chain
				_, err = commands.SendICMMessage(
					[]string{
						subnetName,
						cChain,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from subnet2 to c-chain
				_, err = commands.SendICMMessage(
					[]string{
						subnet2Name,
						cChain,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from c-chain to subnet
				_, err = commands.SendICMMessage(
					[]string{
						cChain,
						subnetName,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from c-chain to subnet2
				_, err = commands.SendICMMessage(
					[]string{
						cChain,
						subnet2Name,
						"hello world",
					},
					utils.TestFlags{
						"key": ewoqKeyName,
					})
				gomega.Expect(err).Should(gomega.BeNil())

				// Send message from subnet to subnet2
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

				// Send message from subnet2 to subnet
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

			ginkgo.It("should deploy the relayer on c-chain and subnet (sov) with funding it from another key", func() {
				// Fund key on subnet
				commandArguments := []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					keyName,
					"--sender-blockchain",
					subnet2Name,
					"--receiver-blockchain",
					subnet2Name,
					"--amount",
					"100",
				}
				_, err := commands.KeyTransferSend(commandArguments)
				gomega.Expect(err).Should(gomega.BeNil())

				// Fund key on c-chain
				commandArguments = []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					keyName,
					"--c-chain-sender",
					"--c-chain-receiver",
					"--amount",
					"100",
				}
				_, err = commands.KeyTransferSend(commandArguments)
				gomega.Expect(err).Should(gomega.BeNil())

				// Deploy ICM contracts
				_, err = commands.DeployICMContracts([]string{}, utils.TestFlags{
					"key":        ewoqKeyName,
					"blockchain": subnet2Name,
				})
				gomega.Expect(err).Should(gomega.BeNil())

				// Deploy relayer
				deployFlags := utils.TestFlags{
					"key":                    key2Name,
					"cchain-amount":          90,
					"blockchains":            subnet2Name,
					"amount":                 50,
					"log-level":              "info",
					"cchain-funding-key":     keyName,
					"blockchain-funding-key": keyName,
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

				subnetFee, err := utils.GetKeyTransferFee(output, subnet2Name)
				gomega.Expect(err).Should(gomega.BeNil())

				cchainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				fmt.Println("output", output)

				// Key2 balance on c-chain
				output, err = commands.ListKeys("local", true, "", "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, key2BalanceCchain, err := utils.ParseAddrBalanceFromKeyListOutput(output, key2Name, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(key2BalanceCchain).Should(gomega.Equal(90 * units.Avax))

				// Key2 balance on subnet
				output, err = commands.ListKeys("local", true, "c,"+subnet2Name, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, key2BalanceSubnet, err := utils.ParseAddrBalanceFromKeyListOutput(output, key2Name, subnet2Name)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(key2BalanceSubnet).Should(gomega.Equal(50 * units.Avax))

				// Key balance on c-chain
				output, err = commands.ListKeys("local", true, "", "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalanceCchain, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(keyBalanceCchain).Should(gomega.Equal(10*units.Avax - cchainFee))

				// Key balance on c-chain
				output, err = commands.ListKeys("local", true, "c,"+subnet2Name, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalanceSubnet, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnet2Name)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(keyBalanceSubnet).Should(gomega.Equal(50*units.Avax - subnetFee))
			})
		})
	})

	ginkgo.Context("With invalid input", func() {
		ginkgo.BeforeEach(func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)
		})

		ginkgo.AfterEach(func() {
			err := utils.DeleteConfigs(subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			utils.DeleteCustomBinary(subnetName)
		})

		ginkgo.It("should fail if relayer is already deployed", func() {
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

			_, err = utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("there is already a local relayer deployed"))
		})

		ginkgo.It("should fail if log level is invalid", func() {
			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":       keyName,
				"log-level": "test",
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("invalid log level test"))
		})

		ginkgo.It("should fail if funding key has no balance on C-Chain", func() {
			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":                keyName,
				"blockchains":        subnetName,
				"amount":             10000,
				"cchain-amount":      10000,
				"log-level":          "info",
				"cchain-funding-key": key2Name,
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("destination C-Chain funding key has no balance"))
		})

		ginkgo.It("should fail if funding key on c-chain has not enough funds", func() {
			// Fund key on c-chain
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				key2Name,
				"--c-chain-sender",
				"--c-chain-receiver",
				"--amount",
				"100",
			}
			_, err := commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy ICM contracts
			_, err = commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":                keyName,
				"blockchains":        subnetName,
				"amount":             10000,
				"cchain-amount":      10000,
				"log-level":          "info",
				"cchain-funding-key": key2Name,
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("desired balance 10000.000000 for destination C-Chain exceeds available funding balance of 99.990000"))
		})

		ginkgo.It("should fail if funding key on subnet has not enough funds", func() {
			// Deploy ICM contracts
			_, err := commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":                    keyName,
				"blockchains":            subnetName,
				"amount":                 10000,
				"cchain-amount":          10000,
				"log-level":              "info",
				"blockchain-funding-key": key2Name,
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("destination %s funding key has no balance", subnetName)))
		})

		ginkgo.It("should fail if funding key has no balance on subnet", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				key2Name,
				"--sender-blockchain",
				subnetName,
				"--receiver-blockchain",
				subnetName,
				"--amount",
				"100",
			}
			_, err := commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy ICM contracts
			_, err = commands.DeployICMContracts([]string{}, utils.TestFlags{
				"key":        ewoqKeyName,
				"blockchain": subnetName,
			})
			gomega.Expect(err).Should(gomega.BeNil())

			// Deploy relayer
			deployFlags := utils.TestFlags{
				"key":                    keyName,
				"blockchains":            subnetName,
				"amount":                 10000,
				"cchain-amount":          10000,
				"log-level":              "info",
				"blockchain-funding-key": key2Name,
			}

			deployArgs := []string{
				"deploy",
				"--cchain",
			}

			output, err := utils.TestCommand(utils.InterchainCmd, "relayer", deployArgs, utils.GlobalFlags{
				"local":             true,
				"skip-update-check": true,
			}, deployFlags)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("desired balance 10000.000000 for destination %s exceeds available funding balance of 99.990000", subnetName)))
		})
	})
})

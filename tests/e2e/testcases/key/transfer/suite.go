// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transfer

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/utils/units"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName        = "e2eKey"
	ewoqKeyName    = "ewoq"
	subnetName     = "e2eSubnetTest"
	ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
)

var _ = ginkgo.Describe("[Key] transfer", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.Context("with valid input", func() {
		ginkgo.BeforeEach(func() {
			_, err := commands.CreateKey(keyName)
			gomega.Expect(err).Should(gomega.BeNil())

			commands.StartNetwork()
		})

		ginkgo.AfterEach(func() {
			commands.CleanNetwork()
			err := utils.DeleteConfigs(subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			utils.DeleteCustomBinary(subnetName)
		})

		ginkgo.It("can transfer from P-chain to P-chain with ewoq key and local key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--p-chain-sender",
				"--p-chain-receiver",
				"--amount",
				amountStr,
			}

			output, err := commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			feeNAvax, err := utils.GetKeyTransferFee(output, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(feeNAvax + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))
		})

		ginkgo.It("can transfer from P-chain to C-chain with ewoq key and local key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--p-chain-sender",
				"--c-chain-receiver",
				"--amount",
				amountStr,
			}

			output, err := commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			cChainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(pChainFee + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - cChainFee))
		})

		ginkgo.It("can transfer from C-chain to P-chain with ewoq key and local key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--c-chain-sender",
				"--p-chain-receiver",
				"--amount",
				amountStr,
			}

			// send/receive without recovery
			output, err := commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			cChainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(cChainFee + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - pChainFee))
		})

		ginkgo.It("can transfer from P-chain to X-chain with ewoq key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--p-chain-sender",
				"--x-chain-receiver",
				"--amount",
				amountStr,
			}

			// send/receive without recovery
			output, err := commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "X-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			xChainFee, err := utils.GetKeyTransferFee(output, "X-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "X-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(pChainFee + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - xChainFee))
		})

		ginkgo.It("can transfer from C-chain to C-chain with ewoq key and local key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--c-chain-sender",
				"--c-chain-receiver",
				"--amount",
				amountStr,
			}

			output, err := commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			feeNAvax, err := utils.GetKeyTransferFee(output, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "", "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(feeNAvax + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))
		})

		ginkgo.It("can transfer from Subnet to Subnet with ewoq key", func() {
			amount := 0.2
			amountStr := fmt.Sprintf("%.2f", amount)
			amountNAvax := uint64(amount * float64(units.Avax))
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
				amountStr,
			}

			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, false)
			commands.DeploySubnetLocallyNonSOV(subnetName)

			output, err := commands.ListKeys("local", true, "c,"+subnetName, "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.KeyTransferSend(commandArguments)
			gomega.Expect(err).Should(gomega.BeNil())

			feeNAvax, err := utils.GetKeyTransferFee(output, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())

			output, err = commands.ListKeys("local", true, "c,"+subnetName, "")
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(feeNAvax + amountNAvax).
				Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
			gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))
		})

		ginkgo.It("can transfer from C-Chain to Subnet with ewoq key and local key", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, true)
			commands.DeploySubnetLocallyNonSOV(subnetName)
			_, err := commands.SendICMMessage([]string{"cchain", subnetName, "hello world"}, utils.TestFlags{"key": ewoqKeyName})
			gomega.Expect(err).Should(gomega.BeNil())
			output := commands.DeployERC20Contract("--local", ewoqKeyName, "TEST", "100000", ewoqEVMAddress, "--c-chain")
			erc20Address, err := utils.GetERC20TokenAddress(output)
			gomega.Expect(err).Should(gomega.BeNil())
			icctArgs := []string{
				"--local",
				"--c-chain-home",
				"--remote-blockchain",
				subnetName,
				"--deploy-erc20-home",
				erc20Address,
				"--home-genesis-key",
				"--remote-genesis-key",
			}

			output = commands.DeployInterchainTokenTransferrer(icctArgs)
			gomega.Expect(err).Should(gomega.BeNil())
			homeAddress, remoteAddress, err := utils.GetTokenTransferrerAddresses(output)
			gomega.Expect(err).Should(gomega.BeNil())

			// Get ERC20 balances
			output, err = commands.ListKeys("local", true, "c,"+subnetName, fmt.Sprintf("%s,%s", erc20Address, remoteAddress))
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyERCBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyERCBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())

			amount := uint64(500)
			amountStr := fmt.Sprintf("%d", amount)
			transferArgs := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--c-chain-sender",
				"--receiver-blockchain",
				subnetName,
				"--amount",
				amountStr,
				"--origin-transferrer-address",
				homeAddress,
				"--destination-transferrer-address",
				remoteAddress,
			}

			_, err = commands.KeyTransferSend(transferArgs)
			gomega.Expect(err).Should(gomega.BeNil())

			// Verify ERC20 balances
			output, err = commands.ListKeys("local", true, "c,"+subnetName, fmt.Sprintf("%s,%s", erc20Address, remoteAddress))
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyERCBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(amount).
				Should(gomega.Equal(ewoqKeyERCBalance1 - ewoqKeyERCBalance2))
			gomega.Expect(keyBalance2 - keyERCBalance1).Should(gomega.Equal(amount))
		})

		ginkgo.It("can transfer from Subnet to C-chain with ewoq key and local key", func() {
			commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath, true)
			commands.DeploySubnetLocallyNonSOV(subnetName)
			// commands.SendICMMessage("--local", "cchain", subnetName, "hello world", ewoqKeyName)
			output := commands.DeployERC20Contract("--local", ewoqKeyName, "TEST", "100000", ewoqEVMAddress, subnetName)
			erc20Address, err := utils.GetERC20TokenAddress(output)
			gomega.Expect(err).Should(gomega.BeNil())
			icctArgs := []string{
				"--local",
				"--c-chain-remote",
				"--home-blockchain",
				subnetName,
				"--deploy-erc20-home",
				erc20Address,
				"--home-genesis-key",
				"--remote-genesis-key",
			}

			output = commands.DeployInterchainTokenTransferrer(icctArgs)
			gomega.Expect(err).Should(gomega.BeNil())
			homeAddress, remoteAddress, err := utils.GetTokenTransferrerAddresses(output)
			gomega.Expect(err).Should(gomega.BeNil())

			// Get ERC20 balances
			output, err = commands.ListKeys("local", true, "c,"+subnetName, fmt.Sprintf("%s,%s", erc20Address, remoteAddress))
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyERCBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyERCBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())

			amount := uint64(500)
			amountStr := fmt.Sprintf("%d", amount)
			transferArgs := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				keyName,
				"--c-chain-receiver",
				"--sender-blockchain",
				subnetName,
				"--amount",
				amountStr,
				"--origin-transferrer-address",
				homeAddress,
				"--destination-transferrer-address",
				remoteAddress,
			}

			_, err = commands.KeyTransferSend(transferArgs)
			gomega.Expect(err).Should(gomega.BeNil())

			// Verify ERC20 balances
			output, err = commands.ListKeys("local", true, "c,"+subnetName, fmt.Sprintf("%s,%s", erc20Address, remoteAddress))
			gomega.Expect(err).Should(gomega.BeNil())
			_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
			gomega.Expect(err).Should(gomega.BeNil())
			_, ewoqKeyERCBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(amount).
				Should(gomega.Equal(ewoqKeyERCBalance1 - ewoqKeyERCBalance2))
			gomega.Expect(keyBalance2 - keyERCBalance1).Should(gomega.Equal(amount))
		})
	})
	ginkgo.Context("with invalid input", func() {
		ginkgo.It("should fail when both key and ledger index were provided", func() {
			commandArguments := []string{
				"--local",
				"--key",
				"test",
				"--ledger",
				"10",
				"--destination-key",
				"test",
				"--amount",
				"0.1",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("only one between a keyname or a ledger index must be given"))
		})

		ginkgo.Context("Within intraEvmSend", func() {
			ginkgo.It("should fail when keyName (not ewoq) is provided but no key is found", func() {
				keyName := "nokey"
				commandArguments := []string{
					"--local",
					"--key",
					keyName,
					"--amount",
					"0.1",
					"--c-chain-sender",
					"--c-chain-receiver",
				}
				output, err := commands.KeyTransferSend(commandArguments)

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(output).
					Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/e2e/key/%s.pk: no such file or directory", keyName)))
			})

			ginkgo.It("should fail when destinationKeyName (not ewoq) is provided but no key is found", func() {
				keyName := "nokey"
				commandArguments := []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					keyName,
					"--amount",
					"0.1",
					"--c-chain-sender",
					"--c-chain-receiver",
				}

				output, err := commands.KeyTransferSend(commandArguments)

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(output).
					Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/e2e/key/%s.pk: no such file or directory", keyName)))
			})

			ginkgo.It("should fail when amount provided amount is negative", func() {
				commandArguments := []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					ewoqKeyName,
					"--amount",
					"-0.1",
					"--c-chain-sender",
					"--c-chain-receiver",
				}
				output, err := commands.KeyTransferSend(commandArguments)

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(output).
					Should(gomega.ContainSubstring("amount must be positive"))
			})

			ginkgo.It("should fail to load sidecar when blockchain does not exist in subnets directory", func() {
				blockhainName := "NonExistingBlockchain"
				commandArguments := []string{
					"--local",
					"--key",
					ewoqKeyName,
					"--destination-key",
					ewoqKeyName,
					"--amount",
					"0.1",
					"--sender-blockchain",
					blockhainName,
					"--c-chain-receiver",
				}
				output, err := commands.KeyTransferSend(commandArguments)

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(output).
					Should(gomega.ContainSubstring("failed to load sidecar"))
			})
		})
	})
	ginkgo.Context("with unsupported paths", func() {
		ginkgo.It("should fail when transferring from X-Chain to X-Chain", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--x-chain-sender",
				"--x-chain-receiver",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from X-Chain to X-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from X-Chain to C-Chain", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--x-chain-sender",
				"--c-chain-receiver",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from X-Chain to C-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from X-Chain to P-Chain", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--x-chain-sender",
				"--p-chain-receiver",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from X-Chain to P-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from X-Chain to Subnet", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--x-chain-sender",
				"--receiver-blockchain",
				"Test-Chain",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from X-Chain to Test-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from Subnet to X-Chain", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--x-chain-receiver",
				"--sender-blockchain",
				"Test-Chain",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from Test-Chain to X-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from Subnet to P-Chain", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--p-chain-receiver",
				"--sender-blockchain",
				"Test-Chain",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from Test-Chain to P-Chain is not supported"))
		})

		ginkgo.It("should fail when transferring from P-Chain to Subnet", func() {
			commandArguments := []string{
				"--local",
				"--key",
				ewoqKeyName,
				"--destination-key",
				ewoqKeyName,
				"--amount",
				"0.1",
				"--p-chain-sender",
				"--receiver-blockchain",
				"Test-Chain",
			}
			output, err := commands.KeyTransferSend(commandArguments)

			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).
				Should(gomega.ContainSubstring("transfer from P-Chain to Test-Chain is not supported"))
		})
	})
})

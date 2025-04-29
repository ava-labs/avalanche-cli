// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package key

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName         = "e2eKey"
	keyName2        = "e2eKey2"
	ewoqKeyName     = "ewoq"
	testKey         = "tests/e2e/assets/test_key.pk"
	testKeyWith0x   = "tests/e2e/assets/test_key_0x.pk"
	outputKey       = "/tmp/testKey.pk"
	outputKeywith0x = "/tmp/testKey_0x.pk"
)

var _ = ginkgo.Describe("[Key]", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
	})
	ginkgo.Context("create", func() {
		ginkgo.AfterEach(func() {
			os.Remove(outputKey)
			err := utils.DeleteKey(keyName2)
			gomega.Expect(err).Should(gomega.BeNil())
			os.Remove(outputKeywith0x)
		})

		ginkgo.It("can create a new key", func() {
			// Check config does not already exist
			exists, err := utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())

			output, err := commands.CreateKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			exists, err = utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())
		})

		ginkgo.It("can create a key from file", func() {
			// Check config does not already exist
			exists, err := utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())

			output, err := commands.CreateKeyFromPath(keyName, testKey)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			exists, err = utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())

			// Check two keys are equal
			genKeyPath := path.Join(utils.GetBaseDir(), constants.KeyDir, keyName+constants.KeySuffix)
			equal, err := utils.CheckKeyEquality(testKey, genKeyPath)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})

		ginkgo.It("can create a key from file that contains 0x prefix", func() {
			// Check config does not already exist
			exists, err := utils.KeyExists(keyName2)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())

			output, err := commands.CreateKeyFromPath(keyName2, testKeyWith0x)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			exists, err = utils.KeyExists(keyName2)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())

			// Check two keys are equal
			genKeyPath := path.Join(utils.GetBaseDir(), constants.KeyDir, keyName2+constants.KeySuffix)
			equal, err := utils.CheckKeyEquality(testKey, genKeyPath)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})

		ginkgo.It("can overwrite a key with force", func() {
			// Check config does not already exist
			exists, err := utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())

			output, err := commands.CreateKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Key exists
			exists, err = utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())

			// Create key again, should succeed
			output, err = commands.CreateKeyForce(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())
		})

		ginkgo.It("cannot overwrite a key without force", func() {
			// Check config does not already exist
			exists, err := utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())

			output, err := commands.CreateKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Key exists
			exists, err = utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())

			// Create key again, should fail
			_, err = commands.CreateKey(keyName)
			gomega.Expect(err).Should(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("list", func() {
		ginkgo.It("can list a created key", func() {
			// this could prob be optimized but I think it also helps to clarity
			// if there are independent regexes instead of one large one,
			// difficult to understand (go regexes don't support Perl regex
			// Go RE2 library doesn't support lookahead and lookbehind
			regex1 := `.*NAME.*SUBNET.*ADDRESS.*NETWORK`
			regex2 := `.*e2eKey.*C-Chain.*0x[a-fA-F0-9]{40}`
			regex3 := `.*P-Chain.*[(P-custom)(P-fuji)][a-zA-Z0-9]{39}`
			regex4 := `.*P-custom[a-zA-Z0-9]{39}`

			// Create a key
			output, err := commands.CreateKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Call list cmd
			output, err = commands.ListKeys("local", false, false)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// The matcher for this test is a little weird. Instead of matching an exact
			// string, we check that it matches a regex and contains created keyName. This
			// is to facilitate running the test locally. If you have other keys in your
			// key directory, they will be printed as well. It's impossible to check the
			// list output for exact equality without removing pre-existing user keys.
			// Hence, the matcher here.
			gomega.Expect(output).Should(gomega.MatchRegexp(regex1))
			gomega.Expect(output).Should(gomega.MatchRegexp(regex2))
			gomega.Expect(output).Should(gomega.MatchRegexp(regex3))
			gomega.Expect(output).Should(gomega.MatchRegexp(regex4))
			gomega.Expect(output).Should(gomega.ContainSubstring(keyName))
		})
	})

	ginkgo.Context("export", func() {
		ginkgo.It("can export a key to stdout", func() {
			// Create key
			output, err := commands.CreateKeyFromPath(keyName, testKey)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Export the key
			exportedKey, err := commands.ExportKey(keyName)
			if err != nil {
				fmt.Println(exportedKey)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Trim the trailing newline from the export
			exportedKey = strings.TrimSuffix(exportedKey, "\n")

			// Check two keys are equal
			originalKeyBytes, err := os.ReadFile(testKey)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exportedKey).Should(gomega.Equal(string(originalKeyBytes)))
		})

		ginkgo.It("can export a key to file", func() {
			// Check output key does not already exist
			_, err := os.Stat(outputKey)
			gomega.Expect(err).Should(gomega.HaveOccurred())

			// Create key
			output, err := commands.CreateKeyFromPath(keyName, testKey)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Export the key
			output, err = commands.ExportKeyToFile(keyName, outputKey)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Check two keys are equal
			equal, err := utils.CheckKeyEquality(testKey, outputKey)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("delete", func() {
		ginkgo.It("can delete a key", func() {
			output, err := commands.CreateKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Check key exists
			exists, err := utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeTrue())

			// Delete
			output, err = commands.DeleteKey(keyName)
			if err != nil {
				fmt.Println(output)
				utils.PrintStdErr(err)
			}
			gomega.Expect(err).Should(gomega.BeNil())

			// Check no longer exists
			exists, err = utils.KeyExists(keyName)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(exists).Should(gomega.BeFalse())
		})
	})

	ginkgo.Context("transfer", func() {
		ginkgo.Context("With invalid input", func() {
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
						Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/key/%s.pk: no such file or directory", keyName)))
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
						Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/key/%s.pk: no such file or directory", keyName)))
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
		ginkgo.Context("With unsupported paths", func() {
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
})

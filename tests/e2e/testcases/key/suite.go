// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/avalanchego/utils/units"
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
	subnetName      = "e2eSubnetTest"
)

var _ = ginkgo.FDescribe("[Key]", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		os.Remove(outputKey)
		err = utils.DeleteKey(keyName2)
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
		output, err = commands.ListKeys("local", false, false, "")
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

	ginkgo.Describe("transfer", func() {
		ginkgo.Context("With valid input", func() {
			ginkgo.BeforeEach(func() {
				commands.StartNetwork()
				output, err := commands.CreateKey(keyName)
				if err != nil {
					fmt.Println(output)
					utils.PrintStdErr(err)
				}
				gomega.Expect(err).Should(gomega.BeNil())
			})

			ginkgo.AfterEach(func() {
				err := utils.DeleteKey(keyName)
				gomega.Expect(err).Should(gomega.BeNil())
				err = utils.DeleteKey(ewoqKeyName)
				gomega.Expect(err).Should(gomega.BeNil())
				os.Remove(outputKey)

				commands.StopNetwork()
				commands.CleanNetwork()
				err = utils.DeleteConfigs(subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
			})

			ginkgo.It("can transfer from P-chain to P-chain with ewoq key and local key", func() {
				commands.StartNetworkWithVersion("")
				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery

				output, err := commands.ListKeys("local", true, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				keyAddr, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", keyAddr, "", amountStr, "--p-chain-sender", "", "--p-chain-receiver", "", "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				feeNAvax, err := utils.GetKeyTransferFee(output, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", true, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(feeNAvax + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))

				output, err = commands.ListKeys("local", true, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(feeNAvax + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax))
			})

			ginkgo.It("can transfer from P-chain to C-chain with ewoq key and local key", func() {
				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery
				output, err := commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				keyAddr, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", keyAddr, "", amountStr, "--p-chain-sender", "", "--c-chain-receiver", "", "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cChainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(pChainFee + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - cChainFee))

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(pChainFee + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax - cChainFee))
			})

			ginkgo.It("can transfer from C-chain to C-chain with ewoq key and local key", func() {
				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery
				output, err := commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				keyAddr, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", keyAddr, "", amountStr, "--c-chain-sender", "", "--c-chain-receiver", "", "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				feeNAvax, err := utils.GetKeyTransferFee(output, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(feeNAvax + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(feeNAvax + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax))
			})

			ginkgo.It("can transfer from C-chain to P-chain with ewoq key and local key", func() {
				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery
				output, err := commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", "", "", amountStr, "--c-chain-sender", "", "--p-chain-receiver", "", "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cChainFee, err := utils.GetKeyTransferFee(output, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(cChainFee + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - pChainFee))

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "C-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(cChainFee + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax - pChainFee))
			})

			ginkgo.It("can transfer from P-chain to X-chain with ewoq key", func() {
				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery
				output, err := commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "X-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", "", "", amountStr, "--p-chain-sender", "", "--x-chain-receiver", "", "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				xChainFee, err := utils.GetKeyTransferFee(output, "X-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				pChainFee, err := utils.GetKeyTransferFee(output, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "X-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(pChainFee + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax - xChainFee))

				output, err = commands.ListKeys("local", false, true, "")
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "X-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, "P-Chain")
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(pChainFee + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax - xChainFee))
			})

			ginkgo.It("can transfer from Subnet to Subnet with ewoq key and local key", func() {
				commands.CreateSubnetEvmConfigNonSOV(subnetName, utils.SubnetEvmGenesisPath)
				commands.DeploySubnetLocallyNonSOV(subnetName)

				amount := 0.2
				amountStr := fmt.Sprintf("%.2f", amount)
				amountNAvax := uint64(amount * float64(units.Avax))

				// send/receive without recovery
				output, err := commands.ListKeys("local", false, true, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				keyAddr, keyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance1, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())

				cmd := commands.KeyTransferSend(ewoqKeyName, "", keyAddr, "", amountStr, "--sender-blockchain", subnetName, "--receiver-blockchain", subnetName, "", "")
				outputByte, err := cmd.CombinedOutput()
				output = string(outputByte)
				gomega.Expect(err).Should(gomega.BeNil())

				feeNAvax, err := utils.GetKeyTransferFee(output, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())

				output, err = commands.ListKeys("local", false, true, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance2, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(feeNAvax + amountNAvax).
					Should(gomega.Equal(ewoqKeyBalance1 - ewoqKeyBalance2))
				gomega.Expect(keyBalance2 - keyBalance1).Should(gomega.Equal(amountNAvax))

				output, err = commands.ListKeys("local", false, true, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				_, keyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, keyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				_, ewoqKeyBalance3, err := utils.ParseAddrBalanceFromKeyListOutput(output, ewoqKeyName, subnetName)
				gomega.Expect(err).Should(gomega.BeNil())
				gomega.Expect(ewoqKeyBalance1 - ewoqKeyBalance3).
					Should(gomega.Equal(feeNAvax + amountNAvax))
				gomega.Expect(keyBalance3 - keyBalance1).Should(gomega.Equal(amountNAvax))

				// delete custom vm
				utils.DeleteCustomBinary(subnetName)
			})
		})
		ginkgo.Context("With invalid input", func() {
			ginkgo.It("should fail when both key and ledger index were provided", func() {
				cmd := commands.KeyTransferSend("test", "10", "test", "", "0.1", "", "", "", "", "", "")
				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(output).
					Should(gomega.ContainSubstring("only one between a keyname or a ledger index must be given"))
			})

			ginkgo.Context("Within intraEvmSend", func() {
				ginkgo.It("should fail when keyName (not ewoq) is provided but no key is found", func() {
					keyName := "nokey"
					cmd := commands.KeyTransferSend(keyName, "", "", "", "0.1", "--c-chain-sender", "", "--c-chain-receiver", "", "", "")
					output, err := cmd.CombinedOutput()

					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(string(output)).
						Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/key/%s.pk: no such file or directory", keyName)))
				})

				ginkgo.It("should fail when destinationKeyName (not ewoq) is provided but no key is found", func() {
					keyName := "nokey"
					cmd := commands.KeyTransferSend("ewoq", "", "", keyName, "0.1", "--c-chain-sender", "", "--c-chain-receiver", "", "", "")
					output, err := cmd.CombinedOutput()

					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(string(output)).
						Should(gomega.ContainSubstring(fmt.Sprintf(".avalanche-cli/key/%s.pk: no such file or directory", keyName)))
				})

				ginkgo.It("should fail when amount provided amount is negative", func() {
					cmd := commands.KeyTransferSend("ewoq", "", "", "ewoq", "-0.1", "--c-chain-sender", "", "--c-chain-receiver", "", "", "")
					output, err := cmd.CombinedOutput()

					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(string(output)).
						Should(gomega.ContainSubstring("amount must be positive"))
				})

				ginkgo.It("should fail to load sidecard when blockchain does not exist in subnets directory", func() {
					blockhainName := "NonExistingBlockchain"
					cmd := commands.KeyTransferSend("ewoq", "", "", "ewoq", "0.1", "--sender-blockchain", blockhainName, "--c-chain-receiver", "", "", "")
					output, err := cmd.CombinedOutput()

					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(string(output)).
						Should(gomega.ContainSubstring("failed to load sidecar"))
				})
			})
		})
		ginkgo.Context("With unsupported paths", func() {
			ginkgo.It("should fail when transfering from X-Chain to X-Chain", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--x-chain-sender",
					"",
					"--x-chain-receiver",
					"",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from X-Chain to X-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from X-Chain to C-Chain", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--x-chain-sender",
					"",
					"--c-chain-receiver",
					"",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from X-Chain to C-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from X-Chain to P-Chain", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--x-chain-sender",
					"",
					"--p-chain-receiver",
					"",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from X-Chain to P-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from X-Chain to Subnet", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--x-chain-sender",
					"",
					"--receiver-blockchain",
					"Test-Chain",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from X-Chain to Test-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from Subnet to X-Chain", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--sender-blockchain",
					"Test-Chain",
					"--x-chain-receiver",
					"",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from Test-Chain to X-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from Subnet to P-Chain", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--sender-blockchain",
					"Test-Chain",
					"--p-chain-receiver",
					"",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from Test-Chain to P-Chain is not supported"))
			})

			ginkgo.It("should fail when transfering from P-Chain to Subnet", func() {
				cmd := commands.KeyTransferSend(
					"test",
					"",
					"test",
					"",
					"0.1",
					"--p-chain-sender",
					"",
					"--receiver-blockchain",
					"Test-Chain",
					"",
					"",
				)

				output, err := cmd.CombinedOutput()

				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(string(output)).
					Should(gomega.ContainSubstring("transfer from P-Chain to Test-Chain is not supported"))
			})

		})
	})
})

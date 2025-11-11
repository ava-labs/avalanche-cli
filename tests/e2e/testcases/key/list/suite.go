// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package list

import (
	"fmt"
	"os"
	"regexp"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName     = "e2eKey"
	ledger1Seed = "ledger1"
	ledger2Seed = "ledger2"
)

var _ = ginkgo.Describe("[Key] list", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
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
		output, err = commands.ListKeys("local", false, "", "")
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

	ginkgo.It("can list ledger addresses for multiple indices and chains", func() {
		gomega.Expect(os.Getenv("LEDGER_SIM")).Should(gomega.Equal("true"), "ledger list test not designed for real ledgers: please set env var LEDGER_SIM to true")

		// Start ledger simulator once for all tests
		interactionEndCh, ledgerSimEndCh := utils.StartLedgerSim(0, ledger1Seed, false)

		// Test 1: List all chains (P-Chain, C-Chain, X-Chain) for multiple indices
		output, err := commands.ListLedgerKeys("local", []uint{0, 1}, "p,c,x", "")
		gomega.Expect(err).Should(gomega.BeNil())

		// Verify output contains expected headers
		gomega.Expect(output).Should(gomega.ContainSubstring("Kind"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Name"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Subnet"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Address"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Token"))
		gomega.Expect(output).Should(gomega.ContainSubstring("Balance"))

		// Verify ledger indices are shown
		gomega.Expect(output).Should(gomega.ContainSubstring("index 0"))
		gomega.Expect(output).Should(gomega.ContainSubstring("index 1"))

		// Verify all chains are listed
		gomega.Expect(output).Should(gomega.ContainSubstring("P-Chain"))
		gomega.Expect(output).Should(gomega.ContainSubstring("C-Chain"))
		gomega.Expect(output).Should(gomega.ContainSubstring("X-Chain"))

		// Verify kind is "ledger"
		gomega.Expect(output).Should(gomega.ContainSubstring("ledger"))

		// Verify P-Chain addresses have correct format (P-custom...)
		pChainAddrRegex := `P-custom[a-zA-Z0-9]{39}`
		matched, err := regexp.MatchString(pChainAddrRegex, output)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(matched).Should(gomega.BeTrue())

		// Verify X-Chain addresses have correct format (X-custom...)
		xChainAddrRegex := `X-custom[a-zA-Z0-9]{39}`
		matched, err = regexp.MatchString(xChainAddrRegex, output)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(matched).Should(gomega.BeTrue())

		// Verify C-Chain addresses have correct format (0x...)
		cChainAddrRegex := `0x[a-fA-F0-9]{40}`
		matched, err = regexp.MatchString(cChainAddrRegex, output)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(matched).Should(gomega.BeTrue())

		// Test 2: List only P-Chain addresses
		output, err = commands.ListLedgerKeys("local", []uint{0}, "p", "")
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).Should(gomega.ContainSubstring("P-Chain"))
		gomega.Expect(output).Should(gomega.ContainSubstring("index 0"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("C-Chain"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("X-Chain"))

		// Test 3: List only C-Chain addresses
		output, err = commands.ListLedgerKeys("local", []uint{0}, "c", "")
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).Should(gomega.ContainSubstring("C-Chain"))
		gomega.Expect(output).Should(gomega.ContainSubstring("index 0"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("P-Chain"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("X-Chain"))
		matched, err = regexp.MatchString(cChainAddrRegex, output)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(matched).Should(gomega.BeTrue())

		// Test 4: List only X-Chain addresses
		output, err = commands.ListLedgerKeys("local", []uint{0}, "x", "")
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).Should(gomega.ContainSubstring("X-Chain"))
		gomega.Expect(output).Should(gomega.ContainSubstring("index 0"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("P-Chain"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("C-Chain"))
		matched, err = regexp.MatchString(xChainAddrRegex, output)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(matched).Should(gomega.BeTrue())

		// Close ledger simulator
		close(interactionEndCh)
		<-ledgerSimEndCh
	})
})

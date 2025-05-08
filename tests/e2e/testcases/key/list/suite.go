// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package list

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName = "e2eKey"
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
})

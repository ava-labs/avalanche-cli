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
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName   = "e2eKey"
	testKey   = "tests/e2e/assets/test_key.pk"
	outputKey = "/tmp/testKey.pk"
)

var _ = ginkgo.Describe("[Key]", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		os.Remove(outputKey)
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
		regex1 := `.*KEY NAME.*CHAIN.*ADDRESS.*NETWORK`
		regex2 := `.*e2eKey.*C-Chain.*0x[a-fA-F0-9]{40}`
		regex3 := `.*P-Chain.*P-custom[a-zA-Z0-9]{39}`
		regex4 := `.*P-fuji[a-zA-Z0-9]{39}`

		// Create a key
		output, err := commands.CreateKey(keyName)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		// Call list cmd
		output, err = commands.ListKeys()
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
})

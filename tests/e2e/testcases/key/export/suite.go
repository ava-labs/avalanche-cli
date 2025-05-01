// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package export

import (
	"fmt"
	"os"
	"strings"

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

var _ = ginkgo.Describe("[Key] export", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
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
})

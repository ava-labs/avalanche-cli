// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package key

import (
	"fmt"
	"os"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName         = "e2eKey"
	keyName2        = "e2eKey2"
	testKey         = "tests/e2e/assets/test_key.pk"
	testKeyWith0x   = "tests/e2e/assets/test_key_0x.pk"
	outputKey       = "/tmp/testKey.pk"
	outputKeywith0x = "/tmp/testKey_0x.pk"
)

var _ = ginkgo.Describe("[Key] create", func() {
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
})

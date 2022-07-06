package key

import (
	"fmt"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	keyName = "e2eKey"
	testKey = "tests/e2e/assets/test_key.pk"
)

var _ = ginkgo.Describe("[Key]", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKeys(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
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

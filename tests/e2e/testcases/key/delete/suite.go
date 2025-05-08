// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package delete

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

var _ = ginkgo.Describe("[Key] delete", func() {
	ginkgo.AfterEach(func() {
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
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

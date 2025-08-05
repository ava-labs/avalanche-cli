// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package signatureaggregator

import (
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Signature Aggregator]", ginkgo.Ordered, func() {
	ginkgo.BeforeEach(func() {
		commands.StartNetwork()
	})
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
	})
	ginkgo.It("HAPPY PATH: start signature aggregator", func() {
		startSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "start", "--local")
		_, err := startSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())
		app := utils.GetApp()
		runFilePath := app.GetLocalSignatureAggregatorRunPath(models.Local)
		// Check if run file exists and read ports from it
		if _, err := os.Stat(runFilePath); err == nil {
			// File exists, get process details
			_, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, models.NewLocalNetwork())
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("HAPPY PATH: start signature aggregator with specified version", func() {
		startSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "start", "--local", "--signature-aggregator-version", "signature-aggregator-v0.4.3")
		_, err := startSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())
		app := utils.GetApp()
		runFilePath := app.GetLocalSignatureAggregatorRunPath(models.Local)
		// Check if run file exists and read ports from it
		if _, err := os.Stat(runFilePath); err == nil {
			// File exists, get process details
			_, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, models.NewLocalNetwork())
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})
	ginkgo.It("Error Path: start signature aggregator with invalid version", func() {
		startSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "start", "--local", "--signature-aggregator-version", "v0.4.3")
		output, err := startSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("Error: failed to start signature aggregator: failure downloading"))
	})
})

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	_ "github.com/ava-labs/avalanche-cli/tests/e2e/key"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/network"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/packageman"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/root"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/subnet"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestE2e(t *testing.T) {
	if os.Getenv("RUN_E2E") == "" {
		t.Skip("Environment variable RUN_E2E not set; skipping E2E tests")
	}
	gomega.RegisterFailHandler(ginkgo.Fail)
	format.UseStringerRepresentation = true
	ginkgo.RunSpecs(t, "avalanche-cli e2e test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	cmd := exec.Command("./scripts/build.sh")
	out, err := cmd.Output()
	fmt.Println(string(out))
	gomega.Expect(err).Should(gomega.BeNil())
})

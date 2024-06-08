// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/apm"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/errhandling"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/network"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/create"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/devnet"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/monitoring"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/packageman"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/root"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/local"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/public"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/upgrade"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestE2e(t *testing.T) {
	if !utils.IsE2E() {
		t.Skip("Environment variable RUN_E2E not set; skipping E2E tests")
	}
	gomega.RegisterFailHandler(ginkgo.Fail)
	format.UseStringerRepresentation = true
	ginkgo.RunSpecs(t, "avalanche-cli e2e test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	format.MaxLength = 40000
	cmd := exec.Command("./scripts/build.sh")
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	gomega.Expect(err).Should(gomega.BeNil())
	// make sure metrics are not collected for E2e
	metricsCmd := exec.Command("./bin/avalanche", "config", "metrics", "disable")
	out, err = metricsCmd.CombinedOutput()
	fmt.Println(string(out))
	gomega.Expect(err).Should(gomega.BeNil())
})

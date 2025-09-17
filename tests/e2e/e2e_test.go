// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/apm"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/blockchain/configure"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/blockchain/convert"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/blockchain/deploy"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/contract/deploy/validatorManager"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/errhandling"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/icm/deploy"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/icm/sendMsg"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key/create"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key/delete"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key/export"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key/list"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/key/transfer"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/network"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/network/stop"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/create"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/devnet"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/node/monitoring"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/packageman"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/relayer/deploy"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/relayer/logs_cmd"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/relayer/start"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/relayer/stop"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/root"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/signatureaggregator"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/non-sov/local"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/non-sov/public"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/sov/addRemoveValidatorPoA"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/sov/addRemoveValidatorPoS"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/sov/addValidatorLocal"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/sov/etna"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/subnet/sov/local"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/upgrade/non-sov"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/testcases/upgrade/sov"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestE2e(t *testing.T) {
	if !utils.IsE2E() {
		t.Skip("Environment variable RUN_CLI_E2E not set; skipping E2E tests")
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

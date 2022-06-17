package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	// _ "github.com/ava-labs/avalanche-cli/tests/e2e/network"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/root"
	_ "github.com/ava-labs/avalanche-cli/tests/e2e/subnet"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "avalanche-cli e2e test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	cmd := exec.Command("./scripts/build.sh")
	out, err := cmd.Output()
	fmt.Println(string(out))
	gomega.Expect(err).Should(gomega.BeNil())
})

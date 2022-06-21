package root

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Root]", func() {
	ginkgo.It("can print version", func() {
		expectedVersion, err := os.ReadFile("VERSION")
		expectedVersionStr := fmt.Sprintf("avalanche version %s\n", string(expectedVersion))
		gomega.Expect(err).Should(gomega.BeNil())

		/* #nosec G204 */
		cmd := exec.Command(utils.CLIBinary, "--version")
		out, err := cmd.Output()

		gomega.Expect(string(out)).Should(gomega.Equal(expectedVersionStr))
		gomega.Expect(err).Should(gomega.BeNil())
	})
})

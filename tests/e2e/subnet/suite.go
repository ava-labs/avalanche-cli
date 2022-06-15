package subnet

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Subnet]", func() {
	ginkgo.It("can create and delete a subnet config", func() {
		subnetName := "e2eSubnetTest"
		genesis := "tests/e2e/genesis/test_genesis.json"

		// Check config does not already exist
		exists, err := subnetConfigExists(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(exists).Should(gomega.BeFalse())

		// Create config
		cmd := exec.Command(
			utils.CLIBinary,
			utils.SubnetCmd,
			"create",
			"--file",
			genesis,
			"--evm",
			subnetName,
		)
		fmt.Println(cmd.String())
		_, err = cmd.Output()
		gomega.Expect(err).Should(gomega.BeNil())

		// Config should now exist
		exists, err = subnetConfigExists(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(exists).Should(gomega.BeTrue())

		// Now delete config
		cmd2 := exec.Command(utils.CLIBinary, utils.SubnetCmd, "delete", subnetName)
		_, err = cmd2.Output()
		gomega.Expect(err).Should(gomega.BeNil())

		// Config should no longer exist
		exists, err = subnetConfigExists(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(exists).Should(gomega.BeFalse())
	})
})

func subnetConfigExists(subnetName string) (bool, error) {
	genesis := path.Join(utils.GetBaseDir(), subnetName+constants.Genesis_suffix)
	genesisExists := true
	if _, err := os.Stat(genesis); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		genesisExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	sidecar := path.Join(utils.GetBaseDir(), subnetName+constants.Sidecar_suffix)
	sidecarExists := true
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		sidecarExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	// do an xor
	if (genesisExists || sidecarExists) && !(genesisExists && sidecarExists) {
		return false, errors.New("config half exists")
	}
	return genesisExists && sidecarExists, nil
}

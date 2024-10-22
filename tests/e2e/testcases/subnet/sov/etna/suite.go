// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"os"
	"os/exec"
)

const (
	CLIBinary         = "./bin/avalanche"
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
	avalancheGoPath   = "--avalanchego-path"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
)

func deploySubnetToEtnaSOV() (string, map[string]utils.NodeInfo) {
	// deploy
	s := commands.SimulateEtnaDeploySOV(subnetName, keyName, controlKeys)
	fmt.Printf("obtained siulation %s \n", s)
	//subnetID, err := utils.ParsePublicDeployOutput(s)
	//gomega.Expect(err).Should(gomega.BeNil())
	//// add validators to subnet
	//nodeInfos, err := utils.GetNodesInfo()
	//gomega.Expect(err).Should(gomega.BeNil())
	//for _, nodeInfo := range nodeInfos {
	//	start := time.Now().Add(time.Second * 30).UTC().Format("2006-01-02 15:04:05")
	//	_ = commands.SimulateFujiAddValidator(subnetName, keyName, nodeInfo.ID, start, "24h", "20")
	//}
	//// join to copy vm binary and update config file
	//for _, nodeInfo := range nodeInfos {
	//	_ = commands.SimulateFujiJoin(subnetName, nodeInfo.ConfigFile, nodeInfo.PluginDir, nodeInfo.ID)
	//}
	//// get and check whitelisted subnets from config file
	//var whitelistedSubnets string
	//for _, nodeInfo := range nodeInfos {
	//	whitelistedSubnets, err = utils.GetWhitelistedSubnetsFromConfigFile(nodeInfo.ConfigFile)
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	whitelistedSubnetsSlice := strings.Split(whitelistedSubnets, ",")
	//	gomega.Expect(whitelistedSubnetsSlice).Should(gomega.ContainElement(subnetID))
	//}
	//// update nodes whitelisted subnets
	//err = utils.RestartNodesWithWhitelistedSubnets(whitelistedSubnets)
	//gomega.Expect(err).Should(gomega.BeNil())
	//// wait for subnet walidators to be up
	//err = utils.WaitSubnetValidators(subnetID, nodeInfos)
	//gomega.Expect(err).Should(gomega.BeNil())
	//return subnetID, nodeInfos
	return "", nil
}

func CreateEtnaSubnetEvmConfig() {
	// Check config does not already exist
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeFalse())

	// Create config
	cmdArgs := []string{
		CLIBinary,
		"blockchain",
		"create",
		subnetName,
		"--evm",
		"--proof-of-authority",
		"--poa-manager-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--" + constants.SkipUpdateFlag,
	}
	cmd := exec.Command(CLIBinary, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// Config should now exist
	exists, err = utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())
}

func DeployEtnaSubnet(
	subnetName string,
	key string,
	controlKeys string,
) string {
	// Check config exists
	exists, err := utils.SubnetConfigExists(subnetName)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(exists).Should(gomega.BeTrue())

	// Deploy subnet locally
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		subnetName,
		"--etna-devnet",
		"--use-local-machine",
		avalancheGoPath+"="+utils.EtnaAvalancheGoBinaryPath,
		"--num-local-nodes=1",
		"--ewoq",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
	gomega.Expect(err).Should(gomega.BeNil())

	// disable simulation of public network execution paths on a local network
	err = os.Unsetenv(constants.SimulatePublicNetwork)
	gomega.Expect(err).Should(gomega.BeNil())

	return string(output)
}

var _ = ginkgo.Describe("[Etna Subnet SOV]", func() {
	ginkgo.BeforeEach(func() {
		// key
		_ = utils.DeleteKey(keyName)
		output, err := commands.CreateKeyFromPath(keyName, utils.EwoqKeyPath)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		// subnet config
		_ = utils.DeleteConfigs(subnetName)
		_, avagoVersion := commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPath)

		// local network
		commands.StartNetworkWithVersion(avagoVersion)
	})

	ginkgo.AfterEach(func() {
		commands.DestroyLocalNode()
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})

	ginkgo.It("Deploy To Etna Subnet", func() {
		deploySubnetToEtnaSOV()
	})
})

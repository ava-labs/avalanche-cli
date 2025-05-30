// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package addValidator

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	utilsPkg "github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	subnetName    = "testSubnet"
	localNodeName = "testNode"
)

const ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

var _ = ginkgo.Describe("[Blockchain Add Validator]", ginkgo.Ordered, func() {
	var nodeIDStr, publicKey, pop string
	_ = ginkgo.BeforeEach(func() {
		commands.StartNetwork()
		if ginkgo.CurrentSpecReport().LeafNodeText != "HAPPY PATH: add validator with create-local-validator" {
			createValidatorCmd := exec.Command("./bin/avalanche", "node", "local", "start", localNodeName, "--local")
			_, err := createValidatorCmd.CombinedOutput()
			gomega.Expect(err).Should(gomega.BeNil())

			localClusterUris, err := utils.GetLocalClusterUris()
			gomega.Expect(err).Should(gomega.BeNil())
			fmt.Printf("localClusterUris %s \n", localClusterUris)

			nodeIDStr, publicKey, pop, err = utilsPkg.GetNodeID(localClusterUris[0])
			gomega.Expect(err).Should(gomega.BeNil())

			fmt.Printf("nodeIDStr %s \n", nodeIDStr)
			fmt.Printf("publicKey %s \n", publicKey)
			fmt.Printf("pop %s \n", pop)
		}

		// Create test subnet config
		commands.CreateEtnaSubnetEvmConfig(subnetName, ewoqEVMAddress, commands.PoA)
		globalFlags := utils.GlobalFlags{
			"local":             true,
			"skip-update-check": true,
		}
		blockchainCmdArgs := []string{subnetName}
		_, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, nil)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().LeafNodeText != "HAPPY PATH: add validator with create-local-validator" {
			nodeLocalDestroyCmd := exec.Command("./bin/avalanche", "node", "local", "destroy", localNodeName)
			_, err := nodeLocalDestroyCmd.CombinedOutput()
			gomega.Expect(err).Should(gomega.BeNil())
		}
		nodeLocalDestroyCmd := exec.Command("./bin/avalanche", "node", "local", "destroy", fmt.Sprintf("%s-local-node-local-network", subnetName))
		_, err := nodeLocalDestroyCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())
		app := utils.GetApp()
		os.RemoveAll(filepath.Join(app.GetBaseDir(), "local"))
		commands.CleanNetwork()
		// Cleanup test subnet config
		commands.DeleteSubnetConfig(subnetName)
	})
	blockchainCmdArgs := []string{subnetName}
	globalFlags := utils.GlobalFlags{
		"local":                   true,
		"weight":                  20,
		"ewoq":                    true,
		"skip-update-check":       true,
		"balance":                 0.1,
		"remaining-balance-owner": "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
		"disable-owner":           "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
	}
	ginkgo.It("HAPPY PATH: add validator default", func() {
		fmt.Printf("HAPPYnodeIDStr %s \n", nodeIDStr)
		fmt.Printf("HAPPYpublicKey %s \n", publicKey)
		fmt.Printf("HAPPYpop %s \n", pop)
		testFlags := utils.TestFlags{
			"node-id":                 nodeIDStr,
			"bls-public-key":          publicKey,
			"bls-proof-of-possession": pop,
		}
		_, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		if err != nil {
			fmt.Printf("err %s \n", err.Error())
		}
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: add validator with node endpoint", func() {
		currentLocalMachineURI, err := localnet.GetLocalClusterURIs(utils.GetApp(), localNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		testFlags := utils.TestFlags{
			"node-endpoint": currentLocalMachineURI[0],
		}
		_, err = utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		// checkConvertOnlyOutput(output, false)
		gomega.Expect(err).Should(gomega.BeNil())

		// we should have two local machine instances
		localClusterNames, err := utils.GetAllLocalClusterNames()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterNames)).Should(gomega.Equal(2))

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		validators, err := utils.GetCurrentValidatorsLocalAPI(sc.Networks["Local Network"].SubnetID)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(validators)).Should(gomega.Equal(2))

		found := false
		for _, v := range validators {
			if v.NodeID.String() == nodeIDStr {
				gomega.Expect(int(v.Weight)).Should(gomega.Equal(20))
				found = true
				break
			}
		}
		gomega.Expect(found).Should(gomega.Equal(true))
	})

	ginkgo.It("HAPPY PATH: add validator with create-local-validator", func() {
		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(1))
		currentLocalMachineURI := localClusterUris[0]
		testFlags := utils.TestFlags{
			"create-local-validator": true,
		}
		_, err = utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.BeNil())

		// we should have two local machine instances
		localClusterUris, err = utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(2))

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		validators, err := utils.GetCurrentValidatorsLocalAPI(sc.Networks["Local Network"].SubnetID)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(validators)).Should(gomega.Equal(2))
		nodeIDStr, _, _, err = utilsPkg.GetNodeID(currentLocalMachineURI)
		gomega.Expect(err).Should(gomega.BeNil())

		for _, v := range validators {
			if v.NodeID.String() == nodeIDStr { // skip the initial local machine node from blockchain deploy
				gomega.Expect(int(v.Weight)).Should(gomega.Equal(100))
				continue
			}
			fmt.Printf("uint64(v.Weight) %s \n", uint64(v.Weight))
			gomega.Expect(int(v.Weight)).Should(gomega.Equal(20))
		}
	})

	//ginkgo.It("HAPPY PATH: add validator with external signing group", func() {
	//	testFlags := utils.TestFlags{
	//		"generate-node-id":         true,
	//		"num-bootstrap-validators": 1,
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	sc, err := utils.GetSideCar(blockchainCmdArgs[0])
	//	gomega.Expect(err).Should(gomega.BeNil())
	//	numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
	//	gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(1))
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].NodeID).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSProofOfPossession).ShouldNot(gomega.BeNil())
	//	gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSPublicKey).ShouldNot(gomega.BeNil())
	//})

	ginkgo.It("ERROR PATH: add validator with incorrect weight", func() {
		testFlags := utils.TestFlags{
			"weight":                 50,
			"create-local-validator": true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		fmt.Printf("err %s \n", err.Error())
		fmt.Printf("output %s \n", output)
		gomega.Expect(output).Should(gomega.ContainSubstring("exceeds max allowed weight change"))

		// zero weight
		testFlags = utils.TestFlags{
			"weight":                 0,
			"create-local-validator": true,
		}
		output, err = utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("weight has to be greater than 0"))
	})

	ginkgo.It("ERROR PATH: add validator with insufficient balance", func() {
		keyName := "newE2ETestKey"
		exists, err := utils.KeyExists(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(exists).Should(gomega.BeFalse())

		output, err := commands.CreateKey(keyName)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		testFlags := utils.TestFlags{
			"ewoq":                   false,
			"key":                    keyName,
			"create-local-validator": true,
		}
		output, err = utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("error building tx: insufficient funds:"))
		err = utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("ERROR PATH: add validator with unavailable validator manager owner key", func() {
		testFlags := utils.TestFlags{
			"validator-manager-owner": "0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf",
			"create-local-validator":  true,
		}
		output, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("private key for validator manager owner 0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf is not found"))
	})

	//ginkgo.It("ERROR PATH: add validator with both node endpoint and create local validator", func() {
	//	testFlags := utils.TestFlags{
	//		"validator-manager-owner": "0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("failure initializing validator registration: unauthorized owner (tx failed to be submitted)"))
	//})
	//ginkgo.It("ERROR PATH: add validator with node id, bls info provided and create local validator", func() {
	//	testFlags := utils.TestFlags{
	//		"validator-manager-owner": "0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("failure initializing validator registration: unauthorized owner (tx failed to be submitted)"))
	//})
	//ginkgo.It("ERROR PATH: add validator with node id, bls info provided and node endpoint", func() {
	//	testFlags := utils.TestFlags{
	//		"validator-manager-owner": "0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "addValidator", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("failure initializing validator registration: unauthorized owner (tx failed to be submitted)"))
	//})
	//ginkgo.It("ERROR PATH: add validator with sov flags to a non sov remote blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//
	//ginkgo.It("ERROR PATH: add validator with non sov flags to an sov blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with pos flags to a poa blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with rpc flag with argument set", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
	//ginkgo.It("ERROR PATH: add validator with rpc flag to a non sov blockchain", func() {
	//	testFlags := utils.TestFlags{
	//		"avalanchego-version": "invalid_version",
	//	}
	//	output, err := utils.TestCommand(utils.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
	//	gomega.Expect(err).Should(gomega.HaveOccurred())
	//	gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	//})
})

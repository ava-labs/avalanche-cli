// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package packageman

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	CLIBinary         = "./bin/avalanche"
	subnetName        = "e2eSubnetTest"
	keyName           = "ewoq"
	avalancheGoPath   = "--avalanchego-path"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testLocalNodeName = "e2eSubnetTest-local-node"
)

var err error

func createEtnaSubnetEvmConfig() error {
	// Check config does not already exist
	_, err = utils.SubnetConfigExists(subnetName)
	if err != nil {
		return err
	}

	// Create config
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"create",
		subnetName,
		"--evm",
		"--proof-of-authority",
		"--validator-manager-owner",
		ewoqEVMAddress,
		"--proxy-contract-owner",
		ewoqEVMAddress,
		"--production-defaults",
		"--evm-chain-id=99999",
		"--evm-token=TOK",
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	fmt.Println(string(output))
	return err
}

func createSovereignSubnet() (string, string, error) {
	if err := createEtnaSubnetEvmConfig(); err != nil {
		return "", "", err
	}
	// Deploy subnet on etna devnet with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		subnetName,
		"--etna-devnet",
		"--num-local-nodes=1",
		"--ewoq",
		"--convert-only",
		"--change-owner-address",
		ewoqPChainAddress,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		utils.PrintStdErr(err)
	}
	fmt.Println(string(output))
	subnetID, err := utils.ParsePublicDeployOutput(string(output), utils.SubnetIDParseType)
	if err != nil {
		return "", "", err
	}
	blockchainID, err := utils.ParsePublicDeployOutput(string(output), utils.BlockchainIDParseType)
	if err != nil {
		return "", "", err
	}
	return subnetID, blockchainID, err
}

func destroyLocalNode() {
	_, err := os.Stat(testLocalNodeName)
	if os.IsNotExist(err) {
		return
	}
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"destroy",
		testLocalNodeName,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
}

func getBootstrapValidator() ([]*txs.ConvertSubnetToL1Validator, error) {
	infoClient := info.NewClient("http://127.0.0.1:9650")
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	nodeID, proofOfPossession, err := infoClient.GetNodeID(ctx)
	if err != nil {
		return nil, err
	}
	publicKey := "0x" + hex.EncodeToString(proofOfPossession.PublicKey[:])
	pop := "0x" + hex.EncodeToString(proofOfPossession.ProofOfPossession[:])

	bootstrapValidator := models.SubnetValidator{
		NodeID:               nodeID.String(),
		Weight:               constants.BootstrapValidatorWeight,
		Balance:              constants.BootstrapValidatorBalance,
		BLSPublicKey:         publicKey,
		BLSProofOfPossession: pop,
		ChangeOwnerAddr:      ewoqPChainAddress,
	}
	avaGoBootstrapValidators, err := blockchaincmd.ConvertToAvalancheGoSubnetValidator([]models.SubnetValidator{bootstrapValidator})
	if err != nil {
		return nil, err
	}

	return avaGoBootstrapValidators, nil
}

var _ = ginkgo.Describe("[Validator Manager POA Set Up]", ginkgo.Ordered, func() {
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
		destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		destroyLocalNode()
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})
	ginkgo.It("Set Up POA Validator Manager", func() {
		subnetIDStr, blockchainIDStr, err := createSovereignSubnet()
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.TrackLocalEtnaSubnet(testLocalNodeName, subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
		keyPath := path.Join(utils.GetBaseDir(), constants.KeyDir, fmt.Sprintf("subnet_%s_airdrop", subnetName)+constants.KeySuffix)
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		gomega.Expect(err).Should(gomega.BeNil())
		rpcURL := fmt.Sprintf("http://127.0.0.1:9650/ext/bc/%s/rpc", blockchainIDStr)
		client, err := evm.GetClient(rpcURL)
		gomega.Expect(err).Should(gomega.BeNil())
		evm.WaitForChainID(client)

		network := models.NewNetworkFromCluster(models.NewEtnaDevnetNetwork(), testLocalNodeName)
		extraAggregatorPeers, err := blockchaincmd.ConvertURIToPeers([]string{"http://127.0.0.1:9650"})
		gomega.Expect(err).Should(gomega.BeNil())

		subnetID, err := ids.FromString(subnetIDStr)
		gomega.Expect(err).Should(gomega.BeNil())

		blockchainID, err := ids.FromString(blockchainIDStr)
		gomega.Expect(err).Should(gomega.BeNil())

		avaGoBootstrapValidators, err := getBootstrapValidator()
		gomega.Expect(err).Should(gomega.BeNil())
		ownerAddress := common.HexToAddress(ewoqEVMAddress)
		subnetSDK := blockchainSDK.Subnet{
			SubnetID:            subnetID,
			BlockchainID:        blockchainID,
			OwnerAddress:        &ownerAddress,
			RPC:                 rpcURL,
			BootstrapValidators: avaGoBootstrapValidators,
		}

		err = subnetSDK.InitializeProofOfAuthority(network, k.PrivKeyHex(), extraAggregatorPeers, true, logging.Off)
		gomega.Expect(err).Should(gomega.BeNil())
	})
})

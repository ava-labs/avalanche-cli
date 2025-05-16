// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"

	"github.com/ethereum/go-ethereum/common"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	CLIBinary            = "./bin/avalanche"
	keyName              = "ewoq"
	ewoqEVMAddress       = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress    = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	ProxyContractAddress = "0xFEEDC0DE0000000000000000000000000000000"
)

var err error

func createEtnaSubnetEvmConfig() error {
	// Check config does not already exist
	_, err = utils.SubnetConfigExists(utils.BlockchainName)
	if err != nil {
		return err
	}

	// Create config
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"create",
		utils.BlockchainName,
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
	// Deploy subnet on etna local network with local machine as bootstrap validator
	cmd := exec.Command(
		CLIBinary,
		"blockchain",
		"deploy",
		utils.BlockchainName,
		"--local",
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
	_, err := os.Stat(utils.TestLocalNodeName)
	if os.IsNotExist(err) {
		return
	}
	cmd := exec.Command(
		CLIBinary,
		"node",
		"local",
		"destroy",
		utils.TestLocalNodeName,
		"--"+constants.SkipUpdateFlag,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		fmt.Println(string(output))
		utils.PrintStdErr(err)
	}
}

func getBootstrapValidator(uri string) ([]*txs.ConvertSubnetToL1Validator, error) {
	infoClient := info.NewClient(uri)
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
		Balance:              constants.BootstrapValidatorBalanceNanoAVAX,
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
		_ = utils.DeleteConfigs(utils.BlockchainName)
		destroyLocalNode()
	})

	ginkgo.AfterEach(func() {
		destroyLocalNode()
		commands.DeleteSubnetConfig(utils.BlockchainName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
	})
	ginkgo.It("Set Up POA Validator Manager", func() {
		subnetIDStr, blockchainIDStr, err := createSovereignSubnet()
		gomega.Expect(err).Should(gomega.BeNil())
		uris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(uris)).Should(gomega.Equal(1))
		_, err = commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		keyPath := path.Join(utils.GetBaseDir(), constants.KeyDir, fmt.Sprintf("subnet_%s_airdrop", utils.BlockchainName)+constants.KeySuffix)
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		gomega.Expect(err).Should(gomega.BeNil())
		rpcURL := fmt.Sprintf("%s/ext/bc/%s/rpc", uris[0], blockchainIDStr)
		client, err := evm.GetClient(rpcURL)
		gomega.Expect(err).Should(gomega.BeNil())
		err = client.WaitForEVMBootstrapped(0)
		gomega.Expect(err).Should(gomega.BeNil())

		network := models.NewNetworkFromCluster(models.NewLocalNetwork(), utils.TestLocalNodeName)

		extraAggregatorPeers, err := blockchaincmd.ConvertURIToPeers(uris)
		gomega.Expect(err).Should(gomega.BeNil())

		subnetID, err := ids.FromString(subnetIDStr)
		gomega.Expect(err).Should(gomega.BeNil())

		blockchainID, err := ids.FromString(blockchainIDStr)
		gomega.Expect(err).Should(gomega.BeNil())

		avaGoBootstrapValidators, err := getBootstrapValidator(uris[0])
		gomega.Expect(err).Should(gomega.BeNil())
		ownerAddress := common.HexToAddress(ewoqEVMAddress)
		subnetSDK := blockchainSDK.Subnet{
			SubnetID:            subnetID,
			BlockchainID:        blockchainID,
			OwnerAddress:        &ownerAddress,
			RPC:                 rpcURL,
			BootstrapValidators: avaGoBootstrapValidators,
		}

		ctx, cancel := utils.GetSignatureAggregatorContext()
		defer cancel()
		err = subnetSDK.InitializeProofOfAuthority(
			ctx,
			logging.NoLog{},
			network.SDKNetwork(),
			k.PrivKeyHex(),
			extraAggregatorPeers,
			logging.NoLog{},
			ProxyContractAddress,
			true,
		)
		gomega.Expect(err).Should(gomega.BeNil())
	})
})

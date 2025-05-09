// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configure

import (
	"fmt"
	"os"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	defaultRPCTxFeeCap = 100
	newRPCTxFeeCap1    = 101
	newRPCTxFeeCap2    = 102
)

func checkRPCTxFeeCap(nodesInfo map[string]utils.NodeInfo, blockchainID string, expectedRPCTxFeeCap int) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, blockchainID+".log")
		fileBytes, err := os.ReadFile(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(fileBytes).Should(gomega.ContainSubstring("RPCTxFeeCap:%d", expectedRPCTxFeeCap))
		for _, rpcTxFeeCap := range []int{defaultRPCTxFeeCap, newRPCTxFeeCap1, newRPCTxFeeCap2} {
			if rpcTxFeeCap != expectedRPCTxFeeCap {
				gomega.Expect(fileBytes).ShouldNot(gomega.ContainSubstring("RPCTxFeeCap:%d", rpcTxFeeCap))
			}
		}
	}
}

func cleanupLogs(nodesInfo map[string]utils.NodeInfo, blockchainID string) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, blockchainID+".log")
		err := os.Remove(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
	}
}

var _ = ginkgo.Describe("[Blockchain Configure]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeEach(func() {
		commands.CreateEtnaSubnetEvmConfig(utils.BlockchainName, utils.EwoqEVMAddress, commands.PoA)
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		commands.DeleteSubnetConfig(utils.BlockchainName)
	})

	ginkgo.It("invalid blockchain name", func() {
		output, err := commands.BlockchainConfigure(
			"invalidBlockchainName",
			utils.TestFlags{
				"chain-config": "doesNotMatter",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("Invalid blockchain invalidBlockchainName"))
	})

	ginkgo.It("invalid flag", func() {
		output, err := commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"invalid-flag": "doesNotMatter",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("unknown flag: --invalid-flag"))
	})

	ginkgo.It("invalid blockchain conf path", func() {
		output, err := commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"chain-config": "invalidPath",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
	})

	ginkgo.It("check default blockchain config", func() {
		output, err := utils.TestCommand(
			utils.BlockchainCmd,
			"deploy",
			[]string{utils.BlockchainName},
			utils.GlobalFlags{},
			utils.TestFlags{
				"local":             true,
				"skip-icm-deploy":   true,
				"skip-update-check": true,
			},
		)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
		blockchainID, err := utils.ParseBlockchainIDFromOutput(output)
		gomega.Expect(err).Should(gomega.BeNil())
		nodesInfo, err := utils.GetLocalClusterNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		checkRPCTxFeeCap(nodesInfo, blockchainID, defaultRPCTxFeeCap)
	})

	ginkgo.It("set blockchain config", func() {
		// set blockchain config before deploy
		chainConfig := fmt.Sprintf("{\"rpc-tx-fee-cap\": %d}", newRPCTxFeeCap1)
		chainConfigPath, err := utils.CreateTmpFile(constants.ChainConfigFileName, []byte(chainConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"chain-config": chainConfigPath,
			},
		)
		gomega.Expect(err).Should(gomega.BeNil())
		// deploy l1
		output, err := utils.TestCommand(
			utils.BlockchainCmd,
			"deploy",
			[]string{utils.BlockchainName},
			utils.GlobalFlags{},
			utils.TestFlags{
				"local":             true,
				"skip-icm-deploy":   true,
				"skip-update-check": true,
			},
		)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
		blockchainID, err := utils.ParseBlockchainIDFromOutput(output)
		gomega.Expect(err).Should(gomega.BeNil())
		nodesInfo, err := utils.GetLocalClusterNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		// check
		checkRPCTxFeeCap(nodesInfo, blockchainID, newRPCTxFeeCap1)
		// stop
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		// cleanup logs
		cleanupLogs(nodesInfo, blockchainID)
		// change blockchain config
		chainConfig = fmt.Sprintf("{\"rpc-tx-fee-cap\": %d}", newRPCTxFeeCap2)
		chainConfigPath, err = utils.CreateTmpFile(constants.ChainConfigFileName, []byte(chainConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"chain-config": chainConfigPath,
			},
		)
		gomega.Expect(err).Should(gomega.BeNil())
		// start
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
		// check
		checkRPCTxFeeCap(nodesInfo, blockchainID, newRPCTxFeeCap2)
	})
})

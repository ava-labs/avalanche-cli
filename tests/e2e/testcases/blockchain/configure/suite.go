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
	node1CertPath      = "tests/e2e/assets/node1/staker.crt"
	node2CertPath      = "tests/e2e/assets/node2/staker.crt"
	node1TLSPath       = "tests/e2e/assets/node1/staker.key"
	node2TLSPath       = "tests/e2e/assets/node2/staker.key"
	node1BLSPath       = "tests/e2e/assets/node1/signer.key"
	node2BLSPath       = "tests/e2e/assets/node2/signer.key"
	node1ID            = "NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu"
	node2ID            = "NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5"
)

func checkBlockchainConfig(
	nodesInfo map[string]utils.NodeInfo,
	blockchainID string,
	expectedRPCTxFeeCap int,
	nodesRPCTxFeeCap map[string]int,
) {
	for nodeID, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, blockchainID+".log")
		fileBytes, err := os.ReadFile(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		if nodesRPCTxFeeCap != nil {
			var ok bool
			expectedRPCTxFeeCap, ok = nodesRPCTxFeeCap[nodeID]
			gomega.Expect(ok).Should(gomega.BeTrue())
		}
		gomega.Expect(fileBytes).Should(gomega.ContainSubstring("RPCTxFeeCap:%d", expectedRPCTxFeeCap))
		for _, rpcTxFeeCap := range []int{defaultRPCTxFeeCap, newRPCTxFeeCap1, newRPCTxFeeCap2} {
			if rpcTxFeeCap != expectedRPCTxFeeCap {
				gomega.Expect(fileBytes).ShouldNot(gomega.ContainSubstring("RPCTxFeeCap:%d", rpcTxFeeCap))
			}
		}
	}
}

func getBlockchainConfig(rpcTxFeeCap int) string {
	return fmt.Sprintf("{\"rpc-tx-fee-cap\": %d}", rpcTxFeeCap)
}

func getPerNodeChainConfig(nodesRPCTxFeeCap map[string]int) string {
	perNodeChainConfig := "{\n"
	commaStr := ","
	i := 0
	for nodeID, rpcTxFeeCap := range nodesRPCTxFeeCap {
		if i == len(nodesRPCTxFeeCap)-1 {
			commaStr = ""
		}
		perNodeChainConfig += fmt.Sprintf("  \"%s\": {\"rpc-tx-fee-cap\": %d}%s\n", nodeID, rpcTxFeeCap, commaStr)
		i++
	}
	perNodeChainConfig += "}\n"
	return perNodeChainConfig
}

func subnetConfigLog(nodeID string) string {
	if nodeID == "" {
		return "\"validatorOnly\":false,\"allowedNodes\":[]"
	}
	return fmt.Sprintf("\"validatorOnly\":true,\"allowedNodes\":[\"%s\"]", nodeID)
}

func checkSubnetConfig(
	nodesInfo map[string]utils.NodeInfo,
	expectedNodeID string,
) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, "main.log")
		fileBytes, err := os.ReadFile(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(fileBytes).Should(gomega.ContainSubstring(subnetConfigLog(expectedNodeID)))
		for _, unexpectedNodeID := range []string{node1ID, node2ID} {
			if unexpectedNodeID != expectedNodeID {
				gomega.Expect(fileBytes).ShouldNot(gomega.ContainSubstring(subnetConfigLog(unexpectedNodeID)))
			}
		}
	}
}

func getSubnetConfig(nodeID string) string {
	return fmt.Sprintf("{\"validatorOnly\": true, \"allowedNodes\": [\"%s\"]}", nodeID)
}

func cleanupLogs(nodesInfo map[string]utils.NodeInfo, blockchainID string) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, "main.log")
		err := os.Remove(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		logFile = path.Join(nodeInfo.LogDir, blockchainID+".log")
		err = os.Remove(logFile)
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

	ginkgo.It("invalid per node blockchain conf path", func() {
		output, err := commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"per-node-chain-config": "invalidPath",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
	})

	ginkgo.It("invalid subnet conf path", func() {
		output, err := commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"subnet-config": "invalidPath",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
	})

	ginkgo.It("invalid node conf path", func() {
		output, err := commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"node-config": "invalidPath",
			},
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
	})

	ginkgo.It("check default configs", func() {
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
		checkBlockchainConfig(nodesInfo, blockchainID, defaultRPCTxFeeCap, nil)
		checkSubnetConfig(nodesInfo, "")
	})

	ginkgo.It("set blockchain config", func() {
		// set blockchain config before deploy
		chainConfig := getBlockchainConfig(newRPCTxFeeCap1)
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
		checkBlockchainConfig(nodesInfo, blockchainID, newRPCTxFeeCap1, nil)
		// stop
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		// cleanup logs
		cleanupLogs(nodesInfo, blockchainID)
		// change blockchain config
		chainConfig = getBlockchainConfig(newRPCTxFeeCap2)
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
		checkBlockchainConfig(nodesInfo, blockchainID, newRPCTxFeeCap2, nil)
	})

	ginkgo.It("set per node blockchain config", func() {
		// set per node blockchain config before deploy
		nodesRPCTxFeeCap := map[string]int{
			node1ID: newRPCTxFeeCap1,
			node2ID: newRPCTxFeeCap2,
		}
		perNodeChainConfig := getPerNodeChainConfig(nodesRPCTxFeeCap)
		perNodeChainConfigPath, err := utils.CreateTmpFile(constants.PerNodeChainConfigFileName, []byte(perNodeChainConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"per-node-chain-config": perNodeChainConfigPath,
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
				"local":                   true,
				"num-local-nodes":         2,
				"staking-cert-key-path":   node1CertPath + "," + node2CertPath,
				"staking-tls-key-path":    node1TLSPath + "," + node2TLSPath,
				"staking-signer-key-path": node1BLSPath + "," + node2BLSPath,
				"skip-icm-deploy":         true,
				"skip-update-check":       true,
			},
		)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
		blockchainID, err := utils.ParseBlockchainIDFromOutput(output)
		gomega.Expect(err).Should(gomega.BeNil())
		nodesInfo, err := utils.GetLocalClusterNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		// check
		checkBlockchainConfig(nodesInfo, blockchainID, defaultRPCTxFeeCap, nodesRPCTxFeeCap)
		// stop
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		// cleanup logs
		cleanupLogs(nodesInfo, blockchainID)
		// change per node blockchain config
		nodesRPCTxFeeCap = map[string]int{
			node1ID: newRPCTxFeeCap2,
			node2ID: newRPCTxFeeCap1,
		}
		perNodeChainConfig = getPerNodeChainConfig(nodesRPCTxFeeCap)
		perNodeChainConfigPath, err = utils.CreateTmpFile(constants.PerNodeChainConfigFileName, []byte(perNodeChainConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"per-node-chain-config": perNodeChainConfigPath,
			},
		)
		gomega.Expect(err).Should(gomega.BeNil())
		// start
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
		// check
		checkBlockchainConfig(nodesInfo, blockchainID, defaultRPCTxFeeCap, nodesRPCTxFeeCap)
	})

	ginkgo.It("set subnet config", func() {
		// set subnet config before deploy
		subnetConfig := getSubnetConfig(node1ID)
		subnetConfigPath, err := utils.CreateTmpFile(constants.SubnetConfigFileName, []byte(subnetConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"subnet-config": subnetConfigPath,
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
		checkSubnetConfig(nodesInfo, node1ID)
		// stop
		err = commands.StopNetwork()
		gomega.Expect(err).Should(gomega.BeNil())
		// cleanup logs
		cleanupLogs(nodesInfo, blockchainID)
		// change subnet config
		subnetConfig = getSubnetConfig(node2ID)
		subnetConfigPath, err = utils.CreateTmpFile(constants.SubnetConfigFileName, []byte(subnetConfig))
		gomega.Expect(err).Should(gomega.BeNil())
		_, err = commands.BlockchainConfigure(
			utils.BlockchainName,
			utils.TestFlags{
				"subnet-config": subnetConfigPath,
			},
		)
		gomega.Expect(err).Should(gomega.BeNil())
		// start
		out := commands.StartNetwork()
		gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
		// check
		checkSubnetConfig(nodesInfo, node2ID)
	})
})

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configure

import (
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
	acpSupport1        = 111
	acpSupport2        = 112
	node1CertPath      = "tests/e2e/assets/node1/staker.crt"
	node2CertPath      = "tests/e2e/assets/node2/staker.crt"
	node1TLSPath       = "tests/e2e/assets/node1/staker.key"
	node2TLSPath       = "tests/e2e/assets/node2/staker.key"
	node1BLSPath       = "tests/e2e/assets/node1/signer.key"
	node2BLSPath       = "tests/e2e/assets/node2/signer.key"
	node1ID            = "NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu"
	node2ID            = "NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5"
)

// checks that the nodes given by [nodesInfo] have the [expectedRPCTxFeeCap] value set for the subnet evm L1 with ID [blockchainID]
// if [nodesRPCTxFeeCap] is given, it uses [npdesRPCTxFeeCap[nodeID]] instead of [expectedRPCTxFeeCap], to allow checking of different
// configs at different nodes
// bases the check on subnet-evm log files (blockchainID.log)
// also checks that no other test-related rpcTxFeeCap value is present in the logs
func AssertBlockchainConfigIsSet(
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

// checks that the nodes given by [nodesInfo] have the [expectedNodeID] value set for allowedNodes on the subnet configuration
// bases the check on avalanchego log files (main.log)
// also checks that no other test-related nodeID value is present in the logs for allowedNodes
func AssertSubnetConfigIsSet(
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

// checks that the nodes given by [nodesInfo] have the [expectedACPSupport] value set
// bases the check on avalanchego log files (main.log)
// also checks that no other test-related acpSupport value is present in the log
func AssertNodeConfigIsSet(
	nodesInfo map[string]utils.NodeInfo,
	expectedACPSupport int,
) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, "main.log")
		fileBytes, err := os.ReadFile(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		if expectedACPSupport != -1 {
			gomega.Expect(fileBytes).Should(gomega.ContainSubstring("\"acp-support\":%d", expectedACPSupport))
		}
		for _, acpSupport := range []int{acpSupport1, acpSupport2} {
			if acpSupport != expectedACPSupport {
				gomega.Expect(fileBytes).ShouldNot(gomega.ContainSubstring("\"acp-support\":%d", acpSupport))
			}
		}
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

	ginkgo.Context("with invalid input", func() {
		ginkgo.It("should fail to configure blockchain with invalid blockchain name", func() {
			output, err := commands.ConfigureBlockchain(
				"invalidBlockchainName",
				utils.TestFlags{
					"chain-config": "doesNotMatter",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("Invalid blockchain invalidBlockchainName"))
		})

		ginkgo.It("should fail to configure blockchain with invalid flag", func() {
			output, err := commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"invalid-flag": "doesNotMatter",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("unknown flag: --invalid-flag"))
		})

		ginkgo.It("should fail to configure blockchain with invalid blockchain conf path", func() {
			output, err := commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"chain-config": "invalidPath",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
		})

		ginkgo.It("should fail to configure blockchain with invalid per node blockchain conf path", func() {
			output, err := commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"per-node-chain-config": "invalidPath",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
		})

		ginkgo.It("should fail to configure blockchain with invalid subnet conf path", func() {
			output, err := commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"subnet-config": "invalidPath",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
		})

		ginkgo.It("should fail to configure blockchain with invalid node conf path", func() {
			output, err := commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"node-config": "invalidPath",
				},
			)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring("open invalidPath: no such file or directory"))
		})
	})
	ginkgo.Context("with valid input", func() {
		ginkgo.It("default configs are set after deploy", func() {
			output, err := commands.DeployBlockchain(
				utils.BlockchainName,
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
			AssertBlockchainConfigIsSet(nodesInfo, blockchainID, defaultRPCTxFeeCap, nil)
			AssertSubnetConfigIsSet(nodesInfo, "")
			AssertNodeConfigIsSet(nodesInfo, -1)
		})

		ginkgo.It("set blockchain config", func() {
			// set blockchain config before deploy
			chainConfig := getBlockchainConfig(newRPCTxFeeCap1)
			chainConfigPath, err := utils.CreateTmpFile(constants.ChainConfigFileName, []byte(chainConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"chain-config": chainConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// deploy l1
			output, err := commands.DeployBlockchain(
				utils.BlockchainName,
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
			// check a config is set after deploy
			AssertBlockchainConfigIsSet(nodesInfo, blockchainID, newRPCTxFeeCap1, nil)
			// stop
			err = commands.StopNetwork()
			gomega.Expect(err).Should(gomega.BeNil())
			// cleanup logs
			utils.CleanupLogs(nodesInfo, blockchainID)
			// change blockchain config
			chainConfig = getBlockchainConfig(newRPCTxFeeCap2)
			chainConfigPath, err = utils.CreateTmpFile(constants.ChainConfigFileName, []byte(chainConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"chain-config": chainConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// start
			out := commands.StartNetwork()
			gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
			// check a new config is set after restart
			AssertBlockchainConfigIsSet(nodesInfo, blockchainID, newRPCTxFeeCap2, nil)
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
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"per-node-chain-config": perNodeChainConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// deploy l1
			output, err := commands.DeployBlockchain(
				utils.BlockchainName,
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
			// check a config is set after deploy
			AssertBlockchainConfigIsSet(nodesInfo, blockchainID, defaultRPCTxFeeCap, nodesRPCTxFeeCap)
			// stop
			err = commands.StopNetwork()
			gomega.Expect(err).Should(gomega.BeNil())
			// cleanup logs
			utils.CleanupLogs(nodesInfo, blockchainID)
			// change per node blockchain config
			nodesRPCTxFeeCap = map[string]int{
				node1ID: newRPCTxFeeCap2,
				node2ID: newRPCTxFeeCap1,
			}
			perNodeChainConfig = getPerNodeChainConfig(nodesRPCTxFeeCap)
			perNodeChainConfigPath, err = utils.CreateTmpFile(constants.PerNodeChainConfigFileName, []byte(perNodeChainConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"per-node-chain-config": perNodeChainConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// start
			out := commands.StartNetwork()
			gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
			// check a new config is set after restart
			AssertBlockchainConfigIsSet(nodesInfo, blockchainID, defaultRPCTxFeeCap, nodesRPCTxFeeCap)
		})

		ginkgo.It("set subnet config", func() {
			// set subnet config before deploy
			subnetConfig := getSubnetConfig(node1ID)
			subnetConfigPath, err := utils.CreateTmpFile(constants.SubnetConfigFileName, []byte(subnetConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"subnet-config": subnetConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// deploy l1
			output, err := commands.DeployBlockchain(
				utils.BlockchainName,
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
			// check a config is set after deploy
			AssertSubnetConfigIsSet(nodesInfo, node1ID)
			// stop
			err = commands.StopNetwork()
			gomega.Expect(err).Should(gomega.BeNil())
			// cleanup logs
			utils.CleanupLogs(nodesInfo, blockchainID)
			// change subnet config
			subnetConfig = getSubnetConfig(node2ID)
			subnetConfigPath, err = utils.CreateTmpFile(constants.SubnetConfigFileName, []byte(subnetConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"subnet-config": subnetConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// start
			out := commands.StartNetwork()
			gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
			// check a new config is set after restart
			AssertSubnetConfigIsSet(nodesInfo, node2ID)
		})

		ginkgo.It("set node config", func() {
			// set node config before deploy
			nodeConfig := getNodeConfig(acpSupport1)
			nodeConfigPath, err := utils.CreateTmpFile(constants.NodeConfigFileName, []byte(nodeConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"node-config": nodeConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// deploy l1
			output, err := commands.DeployBlockchain(
				utils.BlockchainName,
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
			// check a config is set after deploy
			AssertNodeConfigIsSet(nodesInfo, acpSupport1)
			// stop
			err = commands.StopNetwork()
			gomega.Expect(err).Should(gomega.BeNil())
			// cleanup logs
			utils.CleanupLogs(nodesInfo, blockchainID)
			// change node config
			nodeConfig = getNodeConfig(acpSupport2)
			nodeConfigPath, err = utils.CreateTmpFile(constants.NodeConfigFileName, []byte(nodeConfig))
			gomega.Expect(err).Should(gomega.BeNil())
			_, err = commands.ConfigureBlockchain(
				utils.BlockchainName,
				utils.TestFlags{
					"node-config": nodeConfigPath,
				},
			)
			gomega.Expect(err).Should(gomega.BeNil())
			// start
			out := commands.StartNetwork()
			gomega.Expect(out).Should(gomega.ContainSubstring("Network ready to use"))
			// check a new config is set after restart
			AssertNodeConfigIsSet(nodesInfo, acpSupport2)
		})
	})
})

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanche-network-runner/api"
	"github.com/ethereum/go-ethereum/common"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName       = "e2eSubnetTest"
	secondSubnetName = "e2eSecondSubnetTest"
	confPath         = "tests/e2e/assets/test_avalanche-cli.json"
)

var (
	mapping map[string]string
	err     error
)

var _ = ginkgo.Describe("[Local Subnet SOV]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeAll(func() {
		mapper := utils.NewVersionMapper()
		mapping, err = utils.GetVersionMapping(mapper)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		if err != nil {
			fmt.Println("Clean network error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		if err != nil {
			fmt.Println("Delete config error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		// delete custom vm
		utils.DeleteCustomBinary(subnetName)
		utils.DeleteCustomBinary(secondSubnetName)
	})

	ginkgo.It("can deploy a custom vm subnet to local SOV", func() {
		customVMPath, err := utils.DownloadCustomVMBin(mapping[utils.SoloSubnetEVMKey1])
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CreateCustomVMConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath, customVMPath)
		deployOutput := commands.DeploySubnetLocallyWithVersionSOV(subnetName, mapping[utils.SoloAvagoKey])
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy a SubnetEvm subnet to local SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)
		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can load viper config and setup node properties for local deploy SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)
		deployOutput := commands.DeploySubnetLocallyWithViperConfSOV(subnetName, confPath)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]
		gomega.Expect(rpc).Should(gomega.HavePrefix("http://127.0.0.1:"))

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can't deploy the same subnet twice to local SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)

		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		fmt.Println(deployOutput)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		out, err := commands.DeploySubnetLocallyWithArgsAndOutputSOV(subnetName, "", "")
		gomega.Expect(err).Should(gomega.HaveOccurred())
		deployOutput = string(out)
		rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
		if err == nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(rpcs).Should(gomega.HaveLen(0))
		gomega.Expect(deployOutput).Should(gomega.ContainSubstring("has already been deployed"))
	})

	ginkgo.It("can deploy multiple subnets to local SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)
		commands.CreateSubnetEvmConfigSOV(secondSubnetName, utils.SubnetEvmGenesis2Path)

		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		rpcs1, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs1).Should(gomega.HaveLen(1))

		deployOutput = commands.DeploySubnetLocallySOV(secondSubnetName)
		rpcs2, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs2).Should(gomega.HaveLen(1))

		err = utils.SetHardhatRPC(rpcs1[0])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.SetHardhatRPC(rpcs2[0])
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})

	ginkgo.It("can deploy custom chain config SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmAllowFeeRecpPath)

		addr := "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

		chainConfig := "{\"feeRecipient\": \"" + addr + "\"}"

		// create a chain config in tmp
		file, err := os.CreateTemp("", constants.ChainConfigFileName+"*")
		gomega.Expect(err).Should(gomega.BeNil())
		err = os.WriteFile(file.Name(), []byte(chainConfig), constants.DefaultPerms755)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.ConfigureChainConfig(subnetName, file.Name())

		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		rpc := rpcs[0]
		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		url, err := url.Parse(rpc)
		gomega.Expect(err).Should(gomega.BeNil())
		port, err := strconv.Atoi(url.Port())
		gomega.Expect(err).Should(gomega.BeNil())
		cClient := api.NewEthClient(url.Hostname(), uint(port))

		ethAddr := common.HexToAddress(addr)
		balance, err := cClient.BalanceAt(context.Background(), ethAddr, nil)
		gomega.Expect(err).Should(gomega.BeNil())

		gomega.Expect(balance.Int64()).Should(gomega.Not(gomega.BeZero()))

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can deploy with custom per chain config node SOV", func() {
		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)

		// create per node chain config
		nodesRPCTxFeeCap := map[string]string{
			"node1": "101",
			"node2": "102",
			"node3": "103",
			"node4": "104",
			"node5": "105",
		}
		perNodeChainConfig := "{\n"
		i := 0
		for nodeName, rpcTxFeeCap := range nodesRPCTxFeeCap {
			commaStr := ","
			if i == len(nodesRPCTxFeeCap)-1 {
				commaStr = ""
			}
			perNodeChainConfig += fmt.Sprintf("  \"%s\": {\"rpc-tx-fee-cap\": %s}%s\n", nodeName, rpcTxFeeCap, commaStr)
			i++
		}
		perNodeChainConfig += "}\n"

		// configure the subnet
		file, err := os.CreateTemp("", constants.PerNodeChainConfigFileName+"*")
		gomega.Expect(err).Should(gomega.BeNil())
		err = os.WriteFile(file.Name(), []byte(perNodeChainConfig), constants.DefaultPerms755)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.ConfigurePerNodeChainConfig(subnetName, file.Name())

		// deploy
		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		// get blockchain ID
		rpcParts := strings.Split(rpcs[0], "/")
		gomega.Expect(rpcParts).Should(gomega.HaveLen(7))
		blockchainID := rpcParts[5]

		// verify that plugin logs reflect per node configuration
		nodesInfo, err := utils.GetNodesInfo()
		gomega.Expect(err).Should(gomega.BeNil())
		for nodeName, nodeInfo := range nodesInfo {
			logFile := path.Join(nodeInfo.LogDir, blockchainID+".log")
			fileBytes, err := os.ReadFile(logFile)
			gomega.Expect(err).Should(gomega.BeNil())
			rpcTxFeeCap, ok := nodesRPCTxFeeCap[nodeName]
			gomega.Expect(ok).Should(gomega.BeTrue())
			gomega.Expect(fileBytes).Should(gomega.ContainSubstring("RPCTxFeeCap:%s", rpcTxFeeCap))
		}

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can list a subnet's validators SOV", func() {
		nodeIDs := []string{
			"NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ",
			"NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
		}

		commands.CreateSubnetEvmConfigSOV(subnetName, utils.SubnetEvmGenesisPoaPath)
		deployOutput := commands.DeploySubnetLocallySOV(subnetName)
		_, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		output, err := commands.ListValidators(subnetName, "local")
		gomega.Expect(err).Should(gomega.BeNil())

		for _, nodeID := range nodeIDs {
			gomega.Expect(output).Should(gomega.ContainSubstring(nodeID))
		}

		commands.DeleteSubnetConfig(subnetName)
	})
})

var _ = ginkgo.Describe("[Subnet Compatibility]", func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		if err := utils.DeleteConfigs(subnetName); err != nil {
			fmt.Println("Clean network error:", err)
			gomega.Expect(err).Should(gomega.BeNil())
		}

		if err := utils.DeleteConfigs(secondSubnetName); err != nil {
			fmt.Println("Delete config error:", err)
			gomega.Expect(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("can deploy a subnet-evm with old version", func() {
		// TODO: use a previous subnet evm release once available
		// TODO: also remove hardocding
		subnetEVMVersion := utils.EtnaSubnetEvmVersion
		commands.CreateSubnetEvmConfigWithVersionSOV(subnetName, utils.SubnetEvmGenesisPoaPath, subnetEVMVersion)
		// TODO: use commands.DeploySubnetLocallySOV once having etna release
		deployOutput := commands.DeploySubnetLocallyWithVersionSOV(subnetName, utils.EtnaAvalancheGoVersion)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		err = utils.RunHardhatTests(utils.BaseTest)
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})

	ginkgo.It("can't deploy conflicting vm versions", func() {
		// TODO: These shouldn't be hardcoded either
		subnetEVMVersion1 := utils.EtnaSubnetEvmVersion
		subnetEVMVersion2 := "v0.6.8"

		commands.CreateSubnetEvmConfigWithVersionSOV(subnetName, utils.SubnetEvmGenesisPoaPath, subnetEVMVersion1)
		commands.CreateSubnetEvmConfigWithVersionSOV(secondSubnetName, utils.SubnetEvmGenesis2Path, subnetEVMVersion2)

		// TODO: use commands.DeploySubnetLocallySOV once having etna release
		deployOutput := commands.DeploySubnetLocallyWithVersionSOV(subnetName, utils.EtnaAvalancheGoVersion)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))

		commands.DeploySubnetLocallyExpectErrorSOV(secondSubnetName)

		commands.DeleteSubnetConfig(subnetName)
		commands.DeleteSubnetConfig(secondSubnetName)
	})
})

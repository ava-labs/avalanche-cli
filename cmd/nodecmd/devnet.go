// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/config"
	coreth_params "github.com/ava-labs/coreth/params"
)

// difference between unlock schedule locktime and startime in original genesis
const (
	genesisLocktimeStartimeDelta    = 2836800
	hexa0Str                        = "0x0"
	defaultLocalCChainFundedAddress = "8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	defaultLocalCChainFundedBalance = "0x295BE96E64066972000000"
	allocationCommonEthAddress      = "0xb3d82b1367d362de99ab59a658165aff520cbd4d"
)

func generateCustomCchainGenesis() ([]byte, error) {
	cChainGenesisMap := map[string]interface{}{}
	cChainGenesisMap["config"] = coreth_params.AvalancheLocalChainConfig
	cChainGenesisMap["nonce"] = hexa0Str
	cChainGenesisMap["timestamp"] = hexa0Str
	cChainGenesisMap["extraData"] = "0x00"
	cChainGenesisMap["gasLimit"] = "0x5f5e100"
	cChainGenesisMap["difficulty"] = hexa0Str
	cChainGenesisMap["mixHash"] = "0x0000000000000000000000000000000000000000000000000000000000000000"
	cChainGenesisMap["coinbase"] = "0x0000000000000000000000000000000000000000"
	cChainGenesisMap["alloc"] = map[string]interface{}{
		defaultLocalCChainFundedAddress: map[string]interface{}{
			"balance": defaultLocalCChainFundedBalance,
		},
	}
	cChainGenesisMap["number"] = hexa0Str
	cChainGenesisMap["gasUsed"] = hexa0Str
	cChainGenesisMap["parentHash"] = "0x0000000000000000000000000000000000000000000000000000000000000000"
	return json.Marshal(cChainGenesisMap)
}

func generateCustomGenesis(networkID uint32, walletAddr string, stakingAddr string, nodeIDs []string) ([]byte, error) {
	genesisMap := map[string]interface{}{}

	// cchain
	cChainGenesisBytes, err := generateCustomCchainGenesis()
	if err != nil {
		return nil, err
	}
	genesisMap["cChainGenesis"] = string(cChainGenesisBytes)

	// pchain genesis
	genesisMap["networkID"] = networkID
	startTime := time.Now().Unix()
	genesisMap["startTime"] = startTime
	initialStakers := []map[string]interface{}{}
	for _, nodeID := range nodeIDs {
		initialStaker := map[string]interface{}{
			"nodeID":        nodeID,
			"rewardAddress": walletAddr,
			"delegationFee": 1000000,
		}
		initialStakers = append(initialStakers, initialStaker)
	}
	genesisMap["initialStakeDuration"] = 31536000
	genesisMap["initialStakeDurationOffset"] = 5400
	genesisMap["initialStakers"] = initialStakers
	lockTime := startTime + genesisLocktimeStartimeDelta
	allocations := []interface{}{}
	alloc := map[string]interface{}{
		"avaxAddr":      walletAddr,
		"ethAddr":       allocationCommonEthAddress,
		"initialAmount": 300000000000000000,
		"unlockSchedule": []interface{}{
			map[string]interface{}{"amount": 20000000000000000},
			map[string]interface{}{"amount": 10000000000000000, "locktime": lockTime},
		},
	}
	allocations = append(allocations, alloc)
	alloc = map[string]interface{}{
		"avaxAddr":      stakingAddr,
		"ethAddr":       allocationCommonEthAddress,
		"initialAmount": 0,
		"unlockSchedule": []interface{}{
			map[string]interface{}{"amount": 10000000000000000, "locktime": lockTime},
		},
	}
	allocations = append(allocations, alloc)
	genesisMap["allocations"] = allocations
	genesisMap["initialStakedFunds"] = []interface{}{
		stakingAddr,
	}
	genesisMap["message"] = "{{ fun_quote }}"

	return json.MarshalIndent(genesisMap, "", " ")
}

func setupDevnet(clusterName string) error {
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	cloudHostIDs, err := utils.MapWithError(ansibleHostIDs, func(s string) (string, error) { _, o, err := models.HostAnsibleIDToCloudID(s); return o, err })
	if err != nil {
		return err
	}
	nodeIDs, err := utils.MapWithError(cloudHostIDs, func(s string) (string, error) {
		n, err := getNodeID(app.GetNodeInstanceDirPath(s))
		return n.String(), err
	})
	if err != nil {
		return err
	}

	// set devnet network
	network := models.NewDevnetNetwork(ansibleHosts[ansibleHostIDs[0]].IP, 9650)
	ux.Logger.PrintToUser("Devnet Network Id: %d", network.ID)
	ux.Logger.PrintToUser("Devnet Endpoint: %s", network.Endpoint)

	// get random staking key for devnet genesis
	k, err := key.NewSoft(network.ID)
	if err != nil {
		return err
	}
	stakingAddrStr := k.X()[0]

	// get ewoq key as funded key for devnet genesis
	k, err = key.LoadEwoq(network.ID)
	if err != nil {
		return err
	}
	walletAddrStr := k.X()[0]

	// create genesis file at each node dir
	genesisBytes, err := generateCustomGenesis(network.ID, walletAddrStr, stakingAddrStr, nodeIDs)
	if err != nil {
		return err
	}
	for _, cloudHostID := range cloudHostIDs {
		outFile := filepath.Join(app.GetNodeInstanceDirPath(cloudHostID), "genesis.json")
		if err := os.WriteFile(outFile, genesisBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
	}

	// create avalanchego conf node.json at each node dir
	bootstrapIPs := []string{}
	bootstrapIDs := []string{}
	for i, ansibleHostID := range ansibleHostIDs {
		cloudHostID := cloudHostIDs[i]
		confMap := map[string]interface{}{}
		confMap[config.HTTPHostKey] = ""
		confMap[config.PublicIPKey] = ansibleHosts[ansibleHostID].IP
		confMap[config.NetworkNameKey] = fmt.Sprintf("network-%d", network.ID)
		confMap[config.BootstrapIDsKey] = strings.Join(bootstrapIDs, ",")
		confMap[config.BootstrapIPsKey] = strings.Join(bootstrapIPs, ",")
		confMap[config.GenesisFileKey] = "/home/ubuntu/.avalanchego/configs/genesis.json"
		bootstrapIDs = append(bootstrapIDs, nodeIDs[i])
		bootstrapIPs = append(bootstrapIPs, ansibleHosts[ansibleHostID].IP+":9651")
		confBytes, err := json.MarshalIndent(confMap, "", " ")
		if err != nil {
			return err
		}
		outFile := filepath.Join(app.GetNodeInstanceDirPath(cloudHostID), "node.json")
		if err := os.WriteFile(outFile, confBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
	}

	// update node/s genesis + conf and start
	if err := ansible.RunAnsiblePlaybookSetupDevnet(
		app.GetAnsibleDir(),
		strings.Join(ansibleHostIDs, ","),
		app.GetNodesDir(),
		app.GetAnsibleInventoryDirPath(clusterName),
	); err != nil {
		return err
	}

	// update cluster config with network information
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	clustersConfig.Clusters[clusterName] = models.ClusterConfig{
		Network: network,
		Nodes:   clusterConfig.Nodes,
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

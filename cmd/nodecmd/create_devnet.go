// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/config"
	avago_upgrade "github.com/ava-labs/avalanchego/upgrade"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	coreth_params "github.com/ava-labs/coreth/params"
	"golang.org/x/exp/maps"
)

// difference between unlock schedule locktime and startime in original genesis
const (
	genesisLocktimeStartimeDelta    = 2836800
	hexa0Str                        = "0x0"
	defaultLocalCChainFundedAddress = "8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	defaultLocalCChainFundedBalance = "0x295BE96E64066972000000"
	allocationCommonEthAddress      = "0xb3d82b1367d362de99ab59a658165aff520cbd4d"
)

//go:embed upgrade.json
var upgradeBytes []byte

func generateCustomCchainGenesis() ([]byte, error) {
	cChainGenesisMap := map[string]interface{}{}
	cChainGenesisMap["config"] = coreth_params.GetChainConfig(avago_upgrade.GetConfig(avago_constants.LocalID), coreth_params.AvalancheLocalChainID)
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

func generateCustomGenesis(
	networkID uint32,
	walletAddr string,
	stakingAddr string,
	hosts []*models.Host,
) ([]byte, error) {
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
	for _, host := range hosts {
		nodeDirPath := app.GetNodeInstanceDirPath(host.GetCloudID())
		blsPath := filepath.Join(nodeDirPath, constants.BLSKeyFileName)
		blsKey, err := os.ReadFile(blsPath)
		if err != nil {
			return nil, err
		}
		blsSk, err := bls.SecretKeyFromBytes(blsKey)
		if err != nil {
			return nil, err
		}
		p := signer.NewProofOfPossession(blsSk)
		pk, err := formatting.Encode(formatting.HexNC, p.PublicKey[:])
		if err != nil {
			return nil, err
		}
		pop, err := formatting.Encode(formatting.HexNC, p.ProofOfPossession[:])
		if err != nil {
			return nil, err
		}
		nodeID, err := getNodeID(nodeDirPath)
		if err != nil {
			return nil, err
		}
		initialStaker := map[string]interface{}{
			"nodeID":        nodeID,
			"rewardAddress": walletAddr,
			"delegationFee": 1000000,
			"signer": map[string]interface{}{
				"proofOfPossession": pop,
				"publicKey":         pk,
			},
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

func setupDevnet(clusterName string, hosts []*models.Host, apiNodeIPMap map[string]string) error {
	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(inventoryPath)
	if err != nil {
		return err
	}
	ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	// set devnet network
	endpointIP := ""
	if len(apiNodeIPMap) > 0 {
		endpointIP = maps.Values(apiNodeIPMap)[0]
	} else {
		endpointIP = ansibleHosts[ansibleHostIDs[0]].IP
	}
	endpoint := node.GetAvalancheGoEndpoint(endpointIP)
	network := models.NewDevnetNetwork(endpoint, 0)
	network = models.NewNetworkFromCluster(network, clusterName)

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

	// exclude API nodes from genesis file generation as they will have no stake
	hostsAPI := utils.Filter(hosts, func(h *models.Host) bool {
		return slices.Contains(maps.Keys(apiNodeIPMap), h.GetCloudID())
	})
	hostsWithoutAPI := utils.Filter(hosts, func(h *models.Host) bool {
		return !slices.Contains(maps.Keys(apiNodeIPMap), h.GetCloudID())
	})
	hostsWithoutAPIIDs := utils.Map(hostsWithoutAPI, func(h *models.Host) string { return h.NodeID })

	// create genesis file at each node dir
	genesisBytes, err := generateCustomGenesis(network.ID, walletAddrStr, stakingAddrStr, hostsWithoutAPI)
	if err != nil {
		return err
	}
	// make sure that custom genesis is saved to the subnet dir
	if err := os.WriteFile(app.GetGenesisPath(blockchainName), genesisBytes, constants.WriteReadReadPerms); err != nil {
		return err
	}

	// create avalanchego conf node.json at each node dir
	bootstrapIPs := []string{}
	bootstrapIDs := []string{}
	// append makes sure that hostsWithoutAPI i.e. validators are proccessed first and API nodes will have full list of validators to bootstrap
	for _, host := range append(hostsWithoutAPI, hostsAPI...) {
		confMap := map[string]interface{}{}
		confMap[config.HTTPHostKey] = ""
		confMap[config.PublicIPKey] = host.IP
		confMap[config.NetworkNameKey] = fmt.Sprintf("network-%d", network.ID)
		confMap[config.BootstrapIDsKey] = strings.Join(bootstrapIDs, ",")
		confMap[config.BootstrapIPsKey] = strings.Join(bootstrapIPs, ",")
		confMap[config.GenesisFileKey] = filepath.Join(constants.DockerNodeConfigPath, constants.GenesisFileName)
		confMap[config.UpgradeFileKey] = filepath.Join(constants.DockerNodeConfigPath, constants.UpgradeFileName)
		confMap[config.ProposerVMUseCurrentHeightKey] = constants.DevnetFlagsProposerVMUseCurrentHeight
		confBytes, err := json.MarshalIndent(confMap, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(app.GetNodeInstanceDirPath(host.GetCloudID()), constants.GenesisFileName), genesisBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(app.GetNodeInstanceDirPath(host.GetCloudID()), constants.UpgradeFileName), upgradeBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(app.GetNodeInstanceDirPath(host.GetCloudID()), constants.NodeFileName), confBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
		if slices.Contains(hostsWithoutAPIIDs, host.NodeID) {
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(host.GetCloudID()))
			if err != nil {
				return err
			}
			bootstrapIDs = append(bootstrapIDs, nodeID.String())
			bootstrapIPs = append(bootstrapIPs, fmt.Sprintf("%s:9651", host.IP))
		}
	}
	// update node/s genesis + conf and start
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()

			keyPath := filepath.Join(app.GetNodesDir(), host.GetCloudID())
			if err := ssh.RunSSHSetupDevNet(host, keyPath); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				ux.Logger.RedXToUser(utils.ScriptLog(host.NodeID, "Setup devnet err: %v", err))
				return
			}
			ux.Logger.GreenCheckmarkToUser(utils.ScriptLog(host.NodeID, "Setup devnet"))
		}(&wgResults, host)
	}
	wg.Wait()
	ux.Logger.PrintLineSeparator()
	for _, node := range hosts {
		if wgResults.HasNodeIDWithError(node.NodeID) {
			ux.Logger.RedXToUser("Node %s is ERROR with error: %s", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
		} else {
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(node.GetCloudID()))
			if err != nil {
				return err
			}
			ux.Logger.GreenCheckmarkToUser("Node %s[%s] is SETUP as devnet", node.GetCloudID(), nodeID)
		}
	}
	// stop execution if at least one node failed
	if wgResults.HasErrors() {
		return fmt.Errorf("failed to deploy node(s) %s", wgResults.GetErrorHostMap())
	}
	ux.Logger.PrintLineSeparator()
	ux.Logger.PrintToUser("Devnet Network Id: %s", logging.Green.Wrap(strconv.FormatUint(uint64(network.ID), 10)))
	ux.Logger.PrintToUser("Devnet Endpoint: %s", logging.Green.Wrap(network.Endpoint))
	ux.Logger.PrintLineSeparator()
	// update cluster config with network information
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	clusterConfig.Network = network
	clustersConfig.Clusters[clusterName] = clusterConfig
	return app.WriteClustersConfigFile(&clustersConfig)
}

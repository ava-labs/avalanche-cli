// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/config/node"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"

	dircopy "github.com/otiai10/copy"
	"golang.org/x/exp/maps"
)

type RunningStatus int64

const (
	UndefinedRunningStatus RunningStatus = iota
	NotRunning                           // no network node is running
	PartiallyRunning                     // only part of the network nodes are running
	Running                              // all network nodes are running
)

type NodeSetting struct {
	StakingTLSKey    []byte
	StakingCertKey   []byte
	StakingSignerKey []byte
	HTTPPort         uint64
	StakingPort      uint64
}

// Creates a new tmpnet with the given parameters
// Accepts:
// - setting specific [networkDir] for the network,
// - a list of [nodes] where some of them have pregenerated parameters
// - [genesis] and [upgradeBytes]
// - [bootstrapIPs] and [bootstrapIDs] to be used (if bootstrapping from another custom network)
// - can be bootstrapped or not depending on [bootstrap] setting
func TmpNetCreate(
	ctx context.Context,
	log logging.Logger,
	networkDir string,
	avalancheGoBinPath string,
	pluginDir string,
	networkID uint32,
	bootstrapIPs []string,
	bootstrapIDs []string,
	genesis *genesis.UnparsedConfig,
	upgradeBytes []byte,
	defaultFlags map[string]interface{},
	nodes []*tmpnet.Node,
	bootstrap bool,
) (*tmpnet.Network, error) {
	if len(upgradeBytes) > 0 {
		defaultFlags[config.UpgradeFileContentKey] = base64.StdEncoding.EncodeToString(upgradeBytes)
	}
	network := &tmpnet.Network{
		Nodes:        nodes,
		Dir:          networkDir,
		DefaultFlags: defaultFlags,
		Genesis:      genesis,
		NetworkID:    networkID,
	}
	if err := network.EnsureDefaultConfig(log, avalancheGoBinPath, pluginDir); err != nil {
		return nil, err
	}
	if len(bootstrapIPs) > 0 {
		for _, node := range network.Nodes {
			node.SetNetworkingConfig(bootstrapIDs, bootstrapIPs)
		}
	}
	if err := tmpNetSetBlockchainsConfigDir(network); err != nil {
		return nil, err
	}
	if err := network.Write(); err != nil {
		return nil, err
	}
	var err error
	if bootstrap {
		err = TmpNetBootstrap(ctx, log, networkDir)
	}
	return network, err
}

// Copies a tmpnet from [oldDir] to [newDir], fixing
// configuration so the new network can be bootstrapped
func TmpNetMove(
	oldDir string,
	newDir string,
) error {
	if err := dircopy.Copy(oldDir, newDir); err != nil {
		return fmt.Errorf("failure storing network at %s onto %s: %w", oldDir, newDir, err)
	}
	entries, err := os.ReadDir(newDir)
	if err != nil {
		return fmt.Errorf("failed to read config dir %s: %w", newDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		flagsFile := filepath.Join(newDir, entry.Name(), "flags.json")
		if utils.FileExists(flagsFile) {
			data, err := utils.ReadJSON(flagsFile)
			if err != nil {
				return err
			}
			data[config.DataDirKey] = filepath.Join(newDir, entry.Name())
			data[config.ChainConfigDirKey], err = tmpNetGetNodeBlockchainConfigsDir(newDir, entry.Name())
			if err != nil {
				return err
			}
			if _, ok := data[config.SubnetConfigDirKey]; ok {
				data[config.SubnetConfigDirKey] = filepath.Join(newDir, "subnets")
			}
			if _, ok := data[config.GenesisFileKey]; ok {
				data[config.GenesisFileKey] = filepath.Join(newDir, "genesis.json")
			}
			if err := utils.WriteJSON(flagsFile, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reads in a tmpnet
func GetTmpNetNetwork(networkDir string) (*tmpnet.Network, error) {
	network, err := tmpnet.ReadNetwork(networkDir)
	if err != nil {
		return network, err
	}
	for i := range network.Nodes {
		// ensure that URI and StakingAddress are empty if the process does not exists
		processPath := filepath.Join(networkDir, network.Nodes[i].NodeID.String(), "process.json")
		if bytes, err := os.ReadFile(processPath); errors.Is(err, os.ErrNotExist) {
			network.Nodes[i].URI = ""
			network.Nodes[i].StakingAddress = netip.AddrPort{}
		} else if err != nil {
			return network, fmt.Errorf("failed to read node process context: %w", err)
		} else {
			processContext := node.ProcessContext{}
			if err := json.Unmarshal(bytes, &processContext); err != nil {
				return network, fmt.Errorf("failed to unmarshal node process context: %w", err)
			}
			if _, err := utils.GetProcess(processContext.PID); err != nil {
				network.Nodes[i].URI = ""
				network.Nodes[i].StakingAddress = netip.AddrPort{}
				if err := os.Remove(processPath); err != nil {
					return network, fmt.Errorf("failed to clean up node process context: %w", err)
				}
			}
		}
	}
	networkID, err := GetTmpNetNetworkID(network)
	if err != nil {
		return network, err
	}
	if IsPublicNetwork(networkID) {
		// this is loaded non empty for public networks, and causes genesis flag to be set later on
		network.Genesis = nil
	}
	return network, nil
}

// Bootstrap a previously generated network
// If [avalancheGoBinPath] is given, uses it instead of the persisted one
func TmpNetLoad(
	ctx context.Context,
	log logging.Logger,
	networkDir string,
	avalancheGoBinPath string,
) (*tmpnet.Network, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	if avalancheGoBinPath != "" {
		for i := range network.Nodes {
			network.Nodes[i].RuntimeConfig = &tmpnet.NodeRuntimeConfig{
				AvalancheGoPath: avalancheGoBinPath,
			}
		}
	}
	if err := network.Write(); err != nil {
		return nil, err
	}
	err = TmpNetBootstrap(ctx, log, networkDir)
	return network, err
}

// Stops the given network
func TmpNetStop(
	networkDir string,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	ctx, cancel := sdkutils.GetTimedContext(2 * time.Minute)
	defer cancel()
	return network.Stop(ctx)
}

// Indicates whether the given network has all, part, or none of its nodes running
func GetTmpNetRunningStatus(networkDir string) (RunningStatus, error) {
	status := UndefinedRunningStatus
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return status, err
	}
	bootstrappedCount := 0
	for _, node := range network.Nodes {
		// tmpnet.ReadNetwork reads the process state of the nodes and ensures the
		// node.URI field is populated only if the node is running
		if len(node.URI) > 0 {
			bootstrappedCount++
		}
	}
	switch bootstrappedCount {
	case 0:
		return NotRunning, nil
	case len(network.Nodes):
		return Running, nil
	default:
		return status, nil
	}
}

// Get first node of the network
func GetTmpNetFirstNode(network *tmpnet.Network) (*tmpnet.Node, error) {
	for _, node := range network.Nodes {
		return node, nil
	}
	return nil, fmt.Errorf("no node found on local network at %s", network.Dir)
}

// Get first running node of the network
func GetTmpNetFirstRunningNode(network *tmpnet.Network) (*tmpnet.Node, error) {
	for _, node := range network.Nodes {
		if node.StakingAddress != (netip.AddrPort{}) {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no running node found on local network at %s", network.Dir)
}

// Get a endpoint to operate with the network
func GetTmpNetEndpoint(network *tmpnet.Network) (string, error) {
	node, err := GetTmpNetFirstRunningNode(network)
	if err != nil {
		return "", err
	}
	return node.URI, nil
}

// Waits for the given blockchain to be bootstrapped on network
// Check this for all network nodes that are also validators of the subnet
// If the network does not validate the blockchain at all, it errors
func WaitTmpNetBlockchainBootstrapped(
	ctx context.Context,
	network *tmpnet.Network,
	blockchainID string,
	subnetID ids.ID,
) error {
	if _, ok := ctx.Deadline(); !ok {
		return fmt.Errorf("no deadline given to a blockchain bootstrapping busy wait. endless loop is possible")
	}
	blockchainBootstrapCheckFrequency := time.Second
	for {
		bootstrapped, err := IsTmpNetBlockchainBootstrapped(ctx, network, blockchainID, subnetID)
		if err != nil {
			return err
		}
		if bootstrapped {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(blockchainBootstrapCheckFrequency):
		}
	}
	return nil
}

// Indicates if the given blockchain is bootstrapped on the network
// Check this for all network nodes that are also trackers of the subnet
// If the network does not track the blockchain at all, it errors
func IsTmpNetBlockchainBootstrapped(
	ctx context.Context,
	network *tmpnet.Network,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	queried := 0
	for _, node := range network.Nodes {
		if isTracking, err := IsTmpNetNodeTrackingSubnet([]*tmpnet.Node{node}, subnetID); err != nil {
			return false, err
		} else if !isTracking {
			continue
		}
		infoClient := info.NewClient(node.URI)
		bootstrapped, err := infoClient.IsBootstrapped(ctx, blockchainID)
		if err != nil && !strings.Contains(err.Error(), "there is no chain with alias/ID") {
			return false, err
		}
		if !bootstrapped {
			return false, nil
		}
		queried++
	}
	if queried == 0 {
		return false, fmt.Errorf("no trackers of %s present on network at %s", blockchainID, network.Dir)
	}
	return true, nil
}

// Indicates if any of the [nodes] do track [subnetID]
func IsTmpNetNodeTrackingSubnet(
	nodes []*tmpnet.Node,
	subnetID ids.ID,
) (bool, error) {
	if subnetID == ids.Empty {
		return true, nil
	}
	trackedSubnets, err := GetTmpNetNodesTrackedSubnets(nodes)
	if err != nil {
		return false, err
	}
	return sdkutils.Belongs(trackedSubnets, subnetID), nil
}

// Returns the subnets tracked by [nodes]
func GetTmpNetNodesTrackedSubnets(
	nodes []*tmpnet.Node,
) ([]ids.ID, error) {
	trackedSubnets := []ids.ID{}
	for _, node := range nodes {
		subnets, err := node.Flags.GetStringVal(config.TrackSubnetsKey)
		if err != nil {
			return nil, fmt.Errorf("failure obtaining tracked subnets flag of node %s: %w", node.NodeID, err)
		}
		subnets = strings.TrimSpace(subnets)
		if subnets != "" {
			for _, subnetStr := range strings.Split(subnets, ",") {
				subnet, err := ids.FromString(subnetStr)
				if err != nil {
					return nil, fmt.Errorf("failure parsing subnet ID from tracked subnet %s of node %s: %w", subnetStr, node.NodeID, err)
				}
				if !sdkutils.Belongs(trackedSubnets, subnet) {
					trackedSubnets = append(trackedSubnets, subnet)
				}
			}
		}
	}
	return trackedSubnets, nil
}

// Assign alias [alias]->[blockchainID] to the given [nodes] of [network]
// if none of the nodes validate the blockchain, it errors
func TmpNetSetAlias(
	nodes []*tmpnet.Node,
	blockchainID string,
	alias string,
	subnetID ids.ID,
) error {
	for _, node := range nodes {
		if isTracking, err := IsTmpNetNodeTrackingSubnet([]*tmpnet.Node{node}, subnetID); err != nil {
			return err
		} else if !isTracking {
			continue
		}
		adminClient := admin.NewClient(node.URI)
		ctx, cancel := sdkutils.GetAPIContext()
		defer cancel()
		aliases, err := adminClient.GetChainAliases(ctx, blockchainID)
		if err != nil {
			return err
		}
		if !sdkutils.Belongs(aliases, alias) {
			if err := adminClient.AliasChain(ctx, blockchainID, alias); err != nil {
				return err
			}
		}
	}
	return nil
}

// Assign alias [blockchain.Name]->[blockchain.ID] for all non standard
// blockchains on the [network]
// if the blockchain is not tracked by the network, skips it
func TmpNetSetDefaultAliases(ctx context.Context, networkDir string) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, network, "P", ids.Empty); err != nil {
		return err
	}
	endpoint, err := GetTmpNetEndpoint(network)
	if err != nil {
		return err
	}
	blockchains, err := GetBlockchainInfo(endpoint)
	if err != nil {
		return err
	}
	for _, blockchain := range blockchains {
		if tracking, err := IsTmpNetNodeTrackingSubnet(network.Nodes, blockchain.SubnetID); err != nil {
			return err
		} else if !tracking {
			continue
		}
		if err := WaitTmpNetBlockchainBootstrapped(ctx, network, blockchain.ID.String(), blockchain.SubnetID); err != nil {
			return err
		}
		if err := TmpNetSetAlias(network.Nodes, blockchain.ID.String(), blockchain.Name, blockchain.SubnetID); err != nil {
			return err
		}
	}
	return nil
}

// Install the given VM binary into the appropriate location with the
// appropriate name
func TmpNetInstallVM(
	log logging.Logger,
	network *tmpnet.Network,
	binaryPath string,
	vmID ids.ID,
) error {
	pluginDir, err := network.DefaultFlags.GetStringVal(config.PluginDirKey)
	if err != nil {
		return err
	}
	pluginPath := filepath.Join(pluginDir, vmID.String())
	return utils.SetupExecFile(log, binaryPath, pluginPath)
}

// Set up blockchain config for the given [nodes] of [network]
func TmpNetSetBlockchainConfig(
	network *tmpnet.Network,
	nodes []*tmpnet.Node,
	blockchainID ids.ID,
	blockchainConfig []byte,
) error {
	if err := tmpNetSetBlockchainsConfigDir(network); err != nil {
		return err
	}
	for _, node := range nodes {
		if err := TmpNetSetNodeBlockchainConfig(
			network,
			node.NodeID,
			blockchainID,
			blockchainConfig,
		); err != nil {
			return err
		}
	}
	return nil
}

// Set up blockchain config for the given [nodeID] of the [network]
func TmpNetSetNodeBlockchainConfig(
	network *tmpnet.Network,
	nodeID ids.NodeID,
	blockchainID ids.ID,
	blockchainConfig []byte,
) error {
	configPath := ""
	for _, node := range network.Nodes {
		if node.NodeID != nodeID {
			continue
		}
		blockchainsConfigDir, err := node.Flags.GetStringVal(config.ChainConfigDirKey)
		if err != nil {
			return err
		}
		configPath = filepath.Join(
			blockchainsConfigDir,
			blockchainID.String(),
			"config.json",
		)
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, constants.DefaultPerms755); err != nil {
			return fmt.Errorf("could not create blockchain config directory %s: %w", configDir, err)
		}
	}
	if configPath == "" {
		return fmt.Errorf("failure writing chain config file: node %s not found on network", nodeID)
	}
	return os.WriteFile(configPath, blockchainConfig, constants.WriteReadReadPerms)
}

// Return path to the blockchain configs dir for the given [networkDir] and [nodeID]. If the dir does not
// exists, first creates it.
func tmpNetGetNodeBlockchainConfigsDir(networkDir string, nodeID string) (string, error) {
	nodeBlockchainConfigsDir := filepath.Join(networkDir, nodeID, "configs", "chains")
	if err := os.MkdirAll(nodeBlockchainConfigsDir, constants.DefaultPerms755); err != nil {
		return "", fmt.Errorf("could not create node blockchains config directory %s: %w", nodeBlockchainConfigsDir, err)
	}
	return nodeBlockchainConfigsDir, nil
}

// Set up the blockchain configs dir for all nodes in the [network]
func tmpNetSetBlockchainsConfigDir(network *tmpnet.Network) error {
	for _, node := range network.Nodes {
		nodeBlockchainConfigsDir, err := tmpNetGetNodeBlockchainConfigsDir(network.Dir, node.NodeID.String())
		if err != nil {
			return err
		}
		node.Flags[config.ChainConfigDirKey] = nodeBlockchainConfigsDir
		if err := node.Write(); err != nil {
			return err
		}
	}
	return nil
}

// Set up subnet config for all nodes in the network
func TmpNetSetSubnetConfig(
	network *tmpnet.Network,
	subnetID ids.ID,
	subnetConfig []byte,
) error {
	configPath := filepath.Join(
		network.Dir,
		"subnets",
		subnetID.String()+".json",
	)
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create subnets config directory %s: %w", configDir, err)
	}
	return os.WriteFile(configPath, subnetConfig, constants.WriteReadReadPerms)
}

// Restart given [nodes] of [network]
// If [subnetIDs] are given, configure the nodes to track the subnets
func TmpNetRestartNodes(
	ctx context.Context,
	log logging.Logger,
	printFunc func(msg string, args ...interface{}),
	network *tmpnet.Network,
	nodes []*tmpnet.Node,
	subnetIDs []ids.ID,
) error {
	for _, node := range nodes {
		if len(subnetIDs) > 0 {
			printFunc("Restarting node %s to track newly deployed subnet/s", node.NodeID)
			subnets, err := node.Flags.GetStringVal(config.TrackSubnetsKey)
			if err != nil {
				return err
			}
			subnetsSet := set.Set[string]{}
			subnets = strings.TrimSpace(subnets)
			if subnets != "" {
				subnetsSet = set.Of(strings.Split(subnets, ",")...)
			}
			for _, subnetID := range subnetIDs {
				subnetsSet.Add(subnetID.String())
			}
			subnets = strings.Join(subnetsSet.List(), ",")
			node.Flags[config.TrackSubnetsKey] = subnets
		}
		if err := TmpNetRestartNode(ctx, log, network, node); err != nil {
			return err
		}
	}
	return WaitTmpNetBlockchainBootstrapped(ctx, network, "P", ids.Empty)
}

// Get network bootstrappers to use to connect to the network
func GetTmpNetBootstrappers(
	networkDir string,
	skipNodeID ids.NodeID,
) ([]string, []string, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, nil, err
	}
	bootstrapIPs := []string{}
	bootstrapIDs := []string{}
	for _, node := range network.Nodes {
		if node.NodeID == skipNodeID {
			continue
		}
		if node.StakingAddress == (netip.AddrPort{}) {
			continue
		}
		bootstrapIPs = append(bootstrapIPs, node.StakingAddress.String())
		bootstrapIDs = append(bootstrapIDs, node.NodeID.String())
	}
	return bootstrapIPs, bootstrapIDs, nil
}

// Get network genesis
func GetTmpNetGenesis(
	networkDir string,
) ([]byte, error) {
	return os.ReadFile(filepath.Join(networkDir, "genesis.json"))
}

// Get network upgrade
func GetTmpNetUpgrade(
	networkDir string,
) ([]byte, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	encodedUpgrade, err := network.DefaultFlags.GetStringVal(config.UpgradeFileContentKey)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(encodedUpgrade)
}

// Restart all nodes on [networkDir] to track [subnetID].
// Before that, set up VM binary [vmBinaryPath] and blockchain and subnet config files from
// [blockchainConfig], [subnetConfig], [perNodeBlockchainConfig]
// If [wallet] is given, for non [sovereign] flows, add nodes as non sovereign validators
// Waits until both P-Chain and the blockchain [blockchainID] are bootstrapped
func TmpNetTrackSubnet(
	ctx context.Context,
	log logging.Logger,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
	blockchainName string,
	sovereign bool,
	blockchainID ids.ID,
	subnetID ids.ID,
	vmBinaryPath string,
	blockchainConfig []byte,
	subnetConfig []byte,
	perNodeBlockchainConfig map[ids.NodeID][]byte,
	wallet *primary.Wallet,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	// VM Binary setup
	vmID, err := utils.VMID(blockchainName)
	if err != nil {
		return err
	}
	if err := TmpNetInstallVM(log, network, vmBinaryPath, vmID); err != nil {
		return err
	}
	// Configs
	if subnetConfig != nil {
		if err := TmpNetSetSubnetConfig(
			network,
			subnetID,
			subnetConfig,
		); err != nil {
			return err
		}
	}
	// Set node related conf, add subnet to tracked and restart nodes
	if err := TmpNetTrackBlockchainOnNodes(
		ctx,
		log,
		printFunc,
		network,
		network.Nodes,
		subnetID,
		blockchainID,
		blockchainConfig,
		perNodeBlockchainConfig,
	); err != nil {
		return err
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, network, "P", ids.Empty); err != nil {
		return err
	}
	if !sovereign && wallet != nil {
		if err := TmpNetAddNonSovereignValidators(ctx, network, subnetID, wallet); err != nil {
			return err
		}
		if err := TmpNetWaitNonSovereignValidators(ctx, network, subnetID); err != nil {
			return err
		}
	}
	printFunc("Waiting for blockchain %s to be bootstrapped", blockchainID)
	if err := WaitTmpNetBlockchainBootstrapped(ctx, network, blockchainID.String(), subnetID); err != nil {
		return err
	}
	return nil
}

// Restart given [nodes] of [network] to track [subnetID].
// Before that, set up blockchain config files from [blockchainConfig] and [perNodeBlockchainConfig]
func TmpNetTrackBlockchainOnNodes(
	ctx context.Context,
	log logging.Logger,
	printFunc func(msg string, args ...interface{}),
	network *tmpnet.Network,
	nodes []*tmpnet.Node,
	subnetID ids.ID,
	blockchainID ids.ID,
	blockchainConfig []byte,
	perNodeBlockchainConfig map[ids.NodeID][]byte,
) error {
	if blockchainConfig != nil {
		if err := TmpNetSetBlockchainConfig(
			network,
			nodes,
			blockchainID,
			blockchainConfig,
		); err != nil {
			return err
		}
	}
	nodeIDs := sdkutils.Map(nodes, func(node *tmpnet.Node) ids.NodeID { return node.NodeID })
	for nodeID, blockchainConfig := range perNodeBlockchainConfig {
		if !sdkutils.Belongs(nodeIDs, nodeID) {
			continue
		}
		if err := TmpNetSetNodeBlockchainConfig(
			network,
			nodeID,
			blockchainID,
			blockchainConfig,
		); err != nil {
			return err
		}
	}
	// Add subnet to tracked and restart nodes
	return TmpNetRestartNodes(
		ctx,
		log,
		printFunc,
		network,
		nodes,
		[]ids.ID{subnetID},
	)
}

// Add all network nodes of [network] as non SOV validators of [subnetID], using [wallet] to pay for fees
// If a node is already validator for the subnet, does nothing with it
func TmpNetAddNonSovereignValidators(
	ctx context.Context,
	network *tmpnet.Network,
	subnetID ids.ID,
	wallet *primary.Wallet,
) error {
	endpoint, err := GetTmpNetEndpoint(network)
	if err != nil {
		return err
	}
	pClient := platformvm.NewClient(endpoint)
	vs, err := pClient.GetCurrentValidators(ctx, avagoconstants.PrimaryNetworkID, nil)
	if err != nil {
		return err
	}
	primaryValidatorsEndtime := make(map[ids.NodeID]time.Time)
	for _, v := range vs {
		primaryValidatorsEndtime[v.NodeID] = time.Unix(int64(v.EndTime), 0)
	}
	vs, err = pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return err
	}
	subnetValidators := set.Set[ids.NodeID]{}
	for _, v := range vs {
		subnetValidators.Add(v.NodeID)
	}
	for _, node := range network.Nodes {
		if isValidator := subnetValidators.Contains(node.NodeID); isValidator {
			continue
		}
		if _, err := wallet.P().IssueAddSubnetValidatorTx(
			&txs.SubnetValidator{
				Validator: txs.Validator{
					NodeID: node.NodeID,
					End:    uint64(primaryValidatorsEndtime[node.NodeID].Unix()),
					Wght:   1000,
				},
				Subnet: subnetID,
			},
			common.WithContext(ctx),
			common.WithPollFrequency(100*time.Millisecond),
		); err != nil {
			return err
		}
	}
	return nil
}

// Waits until all the network nodes of [network] are included as validators of [subnetID] as verified
// on GetCurrentValidators P-Chain API call
func TmpNetWaitNonSovereignValidators(ctx context.Context, network *tmpnet.Network, subnetID ids.ID) error {
	checkFrequency := time.Second
	endpoint, err := GetTmpNetEndpoint(network)
	if err != nil {
		return err
	}
	pClient := platformvm.NewClient(endpoint)
	for _, node := range network.Nodes {
		for {
			vs, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
			if err != nil {
				return err
			}
			subnetValidators := set.Set[ids.NodeID]{}
			for _, v := range vs {
				subnetValidators.Add(v.NodeID)
			}
			if subnetValidators.Contains(node.NodeID) {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(checkFrequency):
			}
		}
	}
	return nil
}

// Return a slice of new [numNodes] nodes, setting keys and ports to either fresh values,
// or values present at companion [nodeSettings] slice
// If [trackedSubnets] are given, set up appropriate flag
func GetNewTmpNetNodes(
	numNodes uint32,
	nodeSettings []NodeSetting,
	trackedSubnets []ids.ID,
) ([]*tmpnet.Node, error) {
	if len(nodeSettings) > int(numNodes) {
		return nil, fmt.Errorf("node settings length is bigger than the number of nodes")
	}
	nodes := []*tmpnet.Node{}
	for i := range numNodes {
		node := tmpnet.NewNode("")
		if int(i) < len(nodeSettings) {
			if len(nodeSettings[i].StakingCertKey) > 0 {
				node.Flags[config.StakingCertContentKey] = base64.StdEncoding.EncodeToString(nodeSettings[i].StakingCertKey)
			}
			if len(nodeSettings[i].StakingTLSKey) > 0 {
				node.Flags[config.StakingTLSKeyContentKey] = base64.StdEncoding.EncodeToString(nodeSettings[i].StakingTLSKey)
			}
			if len(nodeSettings[i].StakingSignerKey) > 0 {
				node.Flags[config.StakingSignerKeyContentKey] = base64.StdEncoding.EncodeToString(nodeSettings[i].StakingSignerKey)
			}
			node.Flags[config.HTTPPortKey] = nodeSettings[i].HTTPPort
			node.Flags[config.StakingPortKey] = nodeSettings[i].StakingPort
		} else {
			node.Flags[config.HTTPPortKey] = 0
			node.Flags[config.StakingPortKey] = 0
		}
		if len(trackedSubnets) > 0 {
			trackedSubnetsStr := sdkutils.Map(trackedSubnets, func(i ids.ID) string { return i.String() })
			node.Flags[config.TrackSubnetsKey] = strings.Join(trackedSubnetsStr, ",")
		}
		if err := node.EnsureKeys(); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Copies [node] data into a new fresh node that is to be used in the same network
// Keeps information regarding genesis, upgrade, bootstrappers, tracked subnets, ...
func TmpNetCopyNode(
	node *tmpnet.Node,
) (*tmpnet.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("can't copy nil node")
	}
	flags := maps.Clone(node.Flags)
	for _, flag := range []string{
		config.StakingCertContentKey,
		config.StakingTLSKeyContentKey,
		config.StakingSignerKeyContentKey,
		config.DataDirKey,
		config.HTTPPortKey,
		config.StakingPortKey,
	} {
		delete(flags, flag)
	}
	newNode := tmpnet.Node{
		Flags: flags,
	}
	if err := newNode.EnsureKeys(); err != nil {
		return nil, nil
	}
	return &newNode, nil
}

// Starts all nodes of [networkDir], and waits for P-chain to be bootstrapped
// Then, persists HTTP and Staking ports (changing the config from dynamic
// ports -if set to 0- into persisted ones)
func TmpNetBootstrap(
	ctx context.Context,
	log logging.Logger,
	networkDir string,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	for _, node := range network.Nodes {
		if err := TmpNetStartNode(ctx, log, network, node); err != nil {
			return err
		}
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, network, "P", ids.Empty); err != nil {
		return err
	}
	return TmpNetPersistPorts(network)
}

// Adds the given [node] to the [network] conf, and starts it
// Waits for P-Chain to be bootstrapped, and persists ports for the node
func TmpNetAddNode(
	ctx context.Context,
	log logging.Logger,
	network *tmpnet.Network,
	node *tmpnet.Node,
	httpPort uint32,
	stakingPort uint32,
) error {
	node.Flags[config.HTTPPortKey] = httpPort
	node.Flags[config.StakingPortKey] = stakingPort
	network.Nodes = append(network.Nodes, node)
	if err := network.EnsureNodeConfig(node); err != nil {
		return err
	}
	if err := tmpNetSetBlockchainsConfigDir(network); err != nil {
		return err
	}
	if err := network.Write(); err != nil {
		return err
	}
	if err := TmpNetStartNode(ctx, log, network, node); err != nil {
		return err
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, network, "P", ids.Empty); err != nil {
		return err
	}
	return TmpNetPersistPorts(network)
}

// Enables sybil proyection on [networkDir]
// This is disabled by default on tmpnet for 1-node networks, but is generally
//
//	needed for 1-node clusters that connect to other network
func TmpNetEnableSybilProtection(
	networkDir string,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	network.DefaultFlags[config.SybilProtectionEnabledKey] = true
	for i := range network.Nodes {
		network.Nodes[i].Flags[config.SybilProtectionEnabledKey] = true
	}
	return network.Write()
}

// Persists http and staking ports of a running network
func TmpNetPersistPorts(
	network *tmpnet.Network,
) error {
	for i := range network.Nodes {
		ipPort, err := utils.GetIPPort(network.Nodes[i].URI)
		if err != nil {
			return fmt.Errorf("couldn't parse node URI %s: %w", network.Nodes[i].URI, err)
		}
		network.Nodes[i].Flags[config.HTTPPortKey] = ipPort.Port()
		network.Nodes[i].Flags[config.StakingPortKey] = network.Nodes[i].StakingAddress.Port()
	}
	return network.Write()
}

// Restart given [node] of [network]
func TmpNetRestartNode(
	ctx context.Context,
	log logging.Logger,
	network *tmpnet.Network,
	node *tmpnet.Node,
) error {
	if err := node.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop node %s: %w", node.NodeID, err)
	}
	if err := TmpNetStartNode(ctx, log, network, node); err != nil {
		return fmt.Errorf("failed to start node %s: %w", node.NodeID, err)
	}
	return nil
}

// Starts given [node] of [network]
func TmpNetStartNode(
	ctx context.Context,
	log logging.Logger,
	network *tmpnet.Network,
	node *tmpnet.Node,
) error {
	networkID, err := GetTmpNetNodeNetworkID(node)
	if err != nil {
		return err
	}
	_, ok := node.Flags[config.BootstrapIPsKey]
	if !ok && !IsPublicNetwork(networkID) {
		// it does not have boostrappers set, and it is also not a public network node,
		// so we need to set bootstrappers from the custom network itself
		bootstrapIPs, bootstrapIDs, err := GetTmpNetBootstrappers(network.Dir, node.NodeID)
		if err != nil {
			return err
		}
		node.SetNetworkingConfig(bootstrapIDs, bootstrapIPs)
	}
	if err := node.Write(); err != nil {
		return err
	}
	if err := node.Start(log); err != nil {
		// Attempt to stop an unhealthy node to provide some assurance to the caller
		// that an error condition will not result in a lingering process.
		return errors.Join(err, node.Stop(ctx))
	}
	return nil
}

// Indicates wether a given network ID is for public network
func IsPublicNetwork(networkID uint32) bool {
	return networkID == avagoconstants.FujiID || networkID == avagoconstants.MainnetID
}

// Returns Network ID of [network]
// Using this instead of network.GetNetworkID
// because latest one reads in an empty genesis
// for public networks on some cases, returning an ID of 0
func GetTmpNetNetworkID(network *tmpnet.Network) (uint32, error) {
	node, err := GetTmpNetFirstNode(network)
	if err != nil {
		return 0, err
	}
	return GetTmpNetNodeNetworkID(node)
}

// Returns Network ID of a [node]
func GetTmpNetNodeNetworkID(node *tmpnet.Node) (uint32, error) {
	networkIDStr, err := node.Flags.GetStringVal(config.NetworkNameKey)
	if err != nil {
		return 0, err
	}
	networkID, err := strconv.ParseUint(networkIDStr, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(networkID), nil
}

// Returns avalanchego path persisted at [networkDir]
func GetTmpNetAvalancheGoBinaryPath(networkDir string) (string, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return "", err
	}
	return network.DefaultRuntimeConfig.AvalancheGoPath, nil
}

// when host is public, we avoid [::] but use public IP
func fixURI(uri string, ip string) string {
	return strings.Replace(uri, "[::]", ip, 1)
}

// reads in tmpnet for external reference. preferred over tmpnet version due to URI transformation
func GetTmpNetNetworkWithURIFix(networkDir string) (*tmpnet.Network, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return network, err
	}
	for _, node := range network.Nodes {
		nodeIP, err := node.Flags.GetStringVal(config.PublicIPKey)
		if err != nil {
			return network, err
		}
		node.URI = fixURI(node.URI, nodeIP)
	}
	return network, nil
}

// Get all node URIs of the network. transformates URIs
func GetTmpNetNodeURIsWithFix(
	networkDir string,
) ([]string, error) {
	network, err := GetTmpNetNetworkWithURIFix(networkDir)
	if err != nil {
		return nil, err
	}
	return sdkutils.Map(network.GetNodeURIs(), func(nodeURI tmpnet.NodeURI) string { return nodeURI.URI }), nil
}

// Get paths for most important avalanchego logs that are present on the network nodes
func GetTmpNetAvailableLogs(
	networkDir string,
	blockchainID ids.ID,
	includeCChain bool,
) ([]string, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	prefixes := []string{}
	if blockchainID != ids.Empty {
		prefixes = append(prefixes, blockchainID.String())
	}
	if includeCChain {
		prefixes = append(prefixes, "C")
	}
	prefixes = append(prefixes, "P")
	prefixes = append(prefixes, "main")
	logPaths := []string{}
	for _, node := range network.Nodes {
		for _, prefix := range prefixes {
			logPath := filepath.Join(networkDir, node.NodeID.String(), "logs", prefix+".log")
			if utils.FileExists(logPath) {
				logPaths = append(logPaths, utils.ReplaceUserHomeWithTilde(logPath))
			}
		}
	}
	return logPaths, nil
}

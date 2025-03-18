// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/base64"
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
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
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
// accepts:
// - settint specific[rootDir] for the network,
// - a list of [nodes] where some of them have pregenerated parameters
// - [upgradeBytes] to be used on the network
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
// configuration needed to as the new network can be bootstrapped
func TmpNetMigrate(
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
			data[config.ChainConfigDirKey], err = tmpNetGetNodeBlockchainConfigsDir(newDir, entry.Name())
			if err != nil {
				return err
			}
			data[config.DataDirKey] = filepath.Join(newDir, entry.Name())
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

// reads in tmpnet
func GetTmpNetNetwork(networkDir string) (*tmpnet.Network, error) {
	return tmpnet.ReadNetwork(networkDir)
}

// Bootstrap a previously generated network
// If [avalancheGoBinPath] is given, uses it instead of the previously
// one used
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
	ctx, cancel := sdkutils.GetTimedContext(2 * time.Minute)
	defer cancel()
	return tmpnet.StopNetwork(ctx, networkDir)
}

// Indicates whether the given network has all of its nodes running, part of them, or none
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
// Check this for all network nodes that are also validators of the subnet
// If the network does not validate the blockchain at all, it errors
func IsTmpNetBlockchainBootstrapped(
	ctx context.Context,
	network *tmpnet.Network,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	var (
		err          error
		validatorIDs []ids.NodeID
	)
	if subnetID != ids.Empty {
		validatorIDs, err = GetTmpNetSubnetValidatorIDs(network, subnetID)
		if err != nil {
			return false, err
		}
	}
	queried := 0
	for _, node := range network.Nodes {
		if validatorIDs != nil && !sdkutils.Belongs(validatorIDs, node.NodeID) {
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
		return false, fmt.Errorf("no validators of %s present on network at %s", blockchainID, network.Dir)
	}
	return true, nil
}

// Returns the subnet validator IDs as per P-Chain [GetValidatorsAt]
func GetTmpNetSubnetValidatorIDs(
	network *tmpnet.Network,
	subnetID ids.ID,
) ([]ids.NodeID, error) {
	endpoint, err := GetTmpNetEndpoint(network)
	if err != nil {
		return nil, err
	}
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	validators, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
	if err != nil {
		return nil, err
	}
	return maps.Keys(validators), nil
}

// Verifies if the network validates the subnet at all
func TmpNetHasValidatorsForSubnet(
	network *tmpnet.Network,
	subnetID ids.ID,
) (bool, error) {
	validatorIDs, err := GetTmpNetSubnetValidatorIDs(network, subnetID)
	if err != nil {
		return false, err
	}
	for _, node := range network.Nodes {
		if sdkutils.Belongs(validatorIDs, node.NodeID) {
			return true, nil
		}
	}
	return false, nil
}

// Assign alias [alias]->[blockchainID] on network
// if the network does not validate the blockchain, it errors
func TmpNetSetAlias(
	network *tmpnet.Network,
	nodes []*tmpnet.Node,
	blockchainID string,
	alias string,
	subnetID ids.ID,
) error {
	var (
		err          error
		validatorIDs []ids.NodeID
	)
	if subnetID != ids.Empty {
		validatorIDs, err = GetTmpNetSubnetValidatorIDs(network, subnetID)
		if err != nil {
			return err
		}
	}
	for _, node := range nodes {
		if validatorIDs != nil && !sdkutils.Belongs(validatorIDs, node.NodeID) {
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
// blockchains on the network
// if the blockchain is not validated by the network, skips it
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
		hasValidators, err := TmpNetHasValidatorsForSubnet(network, blockchain.SubnetID)
		if err != nil {
			return err
		}
		if !hasValidators {
			continue
		}
		if err := WaitTmpNetBlockchainBootstrapped(ctx, network, blockchain.ID.String(), blockchain.SubnetID); err != nil {
			return err
		}
		if err := TmpNetSetAlias(network, network.Nodes, blockchain.ID.String(), blockchain.Name, blockchain.SubnetID); err != nil {
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

// Set up blockchain config for all nodes in the network
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

// Set up blockchain config for the given node
// To be implemented after aligning with tmpnet on
// blockchain supporting different confs for different nodes
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

func tmpNetGetNodeBlockchainConfigsDir(networkDir string, nodeID string) (string, error) {
	nodeBlockchainConfigsDir := filepath.Join(networkDir, nodeID, "configs", "chains")
	if err := os.MkdirAll(nodeBlockchainConfigsDir, constants.DefaultPerms755); err != nil {
		return "", fmt.Errorf("could not create node blockchains config directory %s: %w", nodeBlockchainConfigsDir, err)
	}
	return nodeBlockchainConfigsDir, nil
}

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

// Restart all network nodes
// If [subnetIDs] is given, conf the nodes to track the subnets
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
	nodeIDs := utils.Map(nodes, func(node *tmpnet.Node) ids.NodeID { return node.NodeID })
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

// Add all network nodes of [networkDir] as non SOV validators to [subnetID], using [wallet] to pay for fees
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

// Waits until all the network nodes on [networkDir] are included as validators of [subnetID] as verified
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
		}
		if len(trackedSubnets) > 0 {
			trackedSubnetsStr := utils.Map(trackedSubnets, func(i ids.ID) string { return i.String() })
			node.Flags[config.TrackSubnetsKey] = strings.Join(trackedSubnetsStr, ",")
		}
		if err := node.EnsureKeys(); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

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

func IsPublicNetwork(networkID uint32) bool {
	return networkID == avagoconstants.FujiID || networkID == avagoconstants.MainnetID
}

func GetTmpNetNetworkID(networkDir string) (uint32, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return 0, err
	}
	node, err := GetTmpNetFirstNode(network)
	if err != nil {
		return 0, err
	}
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
	network, err := tmpnet.ReadNetwork(networkDir)
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
	return utils.Map(network.GetNodeURIs(), func(nodeURI tmpnet.NodeURI) string { return nodeURI.URI }), nil
}

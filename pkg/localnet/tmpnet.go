// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"net/netip"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	avagonode "github.com/ava-labs/avalanchego/node"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	avagoConstants "github.com/ava-labs/avalanchego/utils/constants"
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

type BootstrappingStatus int64

const (
	UndefinedBootstrappingStatus BootstrappingStatus = iota
	NotBootstrapped                                  // no network node is bootstrapped
	PartiallyBootstrapped                            // only part of the network nodes are bootstrapped
	FullyBootstrapped                                // all network nodes are bootstrapped
)

type BlockchainInfo struct {
	Name     string
	ID       ids.ID
	SubnetID ids.ID
	VMID     ids.ID
}

type NodeSettings struct {
	StakingTLSKey    []byte
	StakingCertKey   []byte
	StakingSignerKey []byte
	HTTPPort         uint64
	P2PPort          uint64
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
			data[config.ChainConfigDirKey] = filepath.Join(newDir, "chains")
			data[config.DataDirKey] = filepath.Join(newDir, entry.Name())
			_, ok := data[config.GenesisFileKey]
			if ok {
				data[config.GenesisFileKey] = filepath.Join(newDir, "genesis.json")
			}
			if err := utils.WriteJSON(flagsFile, data); err != nil {
				return err
			}
		}
	}
	return nil
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
	for i := range network.Nodes {
		network.Nodes[i].RuntimeConfig = &tmpnet.NodeRuntimeConfig{
			AvalancheGoPath: avalancheGoBinPath,
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

// Indicates wether the given network has all of its nodes alive, part of them, or none
func GetTmpNetBootstrappingStatus(networkDir string) (BootstrappingStatus, error) {
	status := UndefinedBootstrappingStatus
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return status, err
	}
	bootstrappedCount := 0
	for _, node := range network.Nodes {
		processPath := filepath.Join(networkDir, node.NodeID.String(), config.DefaultProcessContextFilename)
		if utils.FileExists(processPath) {
			bs, err := os.ReadFile(processPath)
			if err != nil {
				return status, fmt.Errorf("failed to read node process context at %s: %w", processPath, err)
			}
			processContext := avagonode.ProcessContext{}
			if err := json.Unmarshal(bs, &processContext); err != nil {
				return status, fmt.Errorf("failed to unmarshal node process context at %s: %w", processPath, err)
			}
			if _, err := utils.GetProcess(processContext.PID); err == nil {
				status = PartiallyBootstrapped
				bootstrappedCount++
			}
		}
	}
	switch bootstrappedCount {
	case 0:
		return NotBootstrapped, nil
	case len(network.Nodes):
		return FullyBootstrapped, nil
	default:
		return status, nil
	}
}

// when host is public, we avoid [::] but use public IP
func fixURI(uri string, ip string) string {
	return strings.Replace(uri, "[::]", ip, 1)
}

// reads in tmpnet. preferred over tmpnet version due to URI transformation
func GetTmpNetNetwork(networkDir string) (*tmpnet.Network, error) {
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

// Get all node URIs of the network
func GetTmpNetNodeURIs(
	networkDir string,
) ([]string, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	return utils.Map(network.GetNodeURIs(), func(nodeURI tmpnet.NodeURI) string { return nodeURI.URI }), nil
}

// Get first node of the network
func GetTmpNetFirstNode(networkDir string) (*tmpnet.Node, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return nil, err
	}
	if len(network.Nodes) == 0 {
		return nil, fmt.Errorf("no node found on local network at %s", networkDir)
	}
	return network.Nodes[0], nil
}

// Get a endpoint to operate with the network
func GetTmpNetEndpoint(networkDir string) (string, error) {
	node, err := GetTmpNetFirstNode(networkDir)
	if err != nil {
		return "", err
	}
	return node.URI, nil
}

// Gathers blockchain info for all non standard blockchains
func GetTmpNetBlockchainInfo(networkDir string) ([]BlockchainInfo, error) {
	endpoint, err := GetTmpNetEndpoint(networkDir)
	if err != nil {
		return nil, err
	}
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	blockchains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	blockchainsInfo := []BlockchainInfo{}
	for _, blockchain := range blockchains {
		if blockchain.Name == "C-Chain" || blockchain.Name == "X-Chain" {
			continue
		}
		blockchainInfo := BlockchainInfo{
			Name:     blockchain.Name,
			ID:       blockchain.ID,
			SubnetID: blockchain.SubnetID,
			VMID:     blockchain.VMID,
		}
		blockchainsInfo = append(blockchainsInfo, blockchainInfo)
	}
	return blockchainsInfo, nil
}

// Waits for the given blockchain to be bootstrapped on network
// Check this for all network nodes that are also validators of the subnet
// If the network does not validate the blockchain at all, it errors
func WaitTmpNetBlockchainBootstrapped(
	ctx context.Context,
	networkDir string,
	blockchainID string,
	subnetID ids.ID,
) error {
	blockchainBootstrapCheckFrequency := time.Second
	for {
		boostrapped, err := IsTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID, subnetID)
		if err != nil {
			return err
		}
		if boostrapped {
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
	networkDir string,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return false, err
	}
	var validatorIDs []ids.NodeID
	if subnetID != ids.Empty {
		validatorIDs, err = GetTmpNetSubnetValidatorIDs(networkDir, subnetID)
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
		boostrapped, err := infoClient.IsBootstrapped(ctx, blockchainID)
		if err != nil && !strings.Contains(err.Error(), "there is no chain with alias/ID") {
			return false, err
		}
		if !boostrapped {
			return false, nil
		}
		queried++
	}
	if queried == 0 {
		return false, fmt.Errorf("no validators of %s present on network at %s", blockchainID, networkDir)
	}
	return true, nil
}

// Returns the subnet validator IDs as per P-Chain [GetValidatorsAt]
func GetTmpNetSubnetValidatorIDs(
	networkDir string,
	subnetID ids.ID,
) ([]ids.NodeID, error) {
	endpoint, err := GetTmpNetEndpoint(networkDir)
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
	networkDir string,
	subnetID ids.ID,
) (bool, error) {
	validatorIDs, err := GetTmpNetSubnetValidatorIDs(networkDir, subnetID)
	if err != nil {
		return false, err
	}
	network, err := GetTmpNetNetwork(networkDir)
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
	networkDir string,
	blockchainID string,
	alias string,
	subnetID ids.ID,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	var validatorIDs []ids.NodeID
	if subnetID != ids.Empty {
		validatorIDs, err = GetTmpNetSubnetValidatorIDs(networkDir, subnetID)
		if err != nil {
			return err
		}
	}
	for _, node := range network.Nodes {
		if validatorIDs != nil && !sdkutils.Belongs(validatorIDs, node.NodeID) {
			continue
		}
		adminClient := admin.NewClient(node.URI)
		ctx, cancel := sdkutils.GetAPIContext()
		defer cancel()
		if err := adminClient.AliasChain(ctx, blockchainID, alias); err != nil {
			return err
		}
	}
	return nil
}

// Assign alias [blockchain.Name]->[blockchain.ID] for all non standard
// blockchains on the network
// if the blockchain is not validated by the network, skips it
func TmpNetSetDefaultAliases(ctx context.Context, networkDir string) error {
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P", ids.Empty); err != nil {
		return err
	}
	blockchains, err := GetTmpNetBlockchainInfo(networkDir)
	if err != nil {
		return err
	}
	for _, blockchain := range blockchains {
		hasValidators, err := TmpNetHasValidatorsForSubnet(networkDir, blockchain.SubnetID)
		if err != nil {
			return err
		}
		if !hasValidators {
			continue
		}
		if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, blockchain.ID.String(), blockchain.SubnetID); err != nil {
			return err
		}
		if err := TmpNetSetAlias(networkDir, blockchain.ID.String(), blockchain.Name, blockchain.SubnetID); err != nil {
			return err
		}
	}
	return nil
}

// Install the given VM binary into the appropriate location with the
// appropriate name
func TmpNetInstallVM(networkDir string, binaryPath string, vmID ids.ID) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	pluginDir, err := network.DefaultFlags.GetStringVal(config.PluginDirKey)
	if err != nil {
		return err
	}
	pluginPath := filepath.Join(pluginDir, vmID.String())
	if err := utils.FileCopy(binaryPath, pluginPath); err != nil {
		return err
	}
	if err := os.Chmod(pluginPath, constants.DefaultPerms755); err != nil {
		return err
	}
	return nil
}

// Set up blockchain config for all nodes in the network
func TmpNetSetBlockchainConfig(
	networkDir string,
	blockchainID ids.ID,
	blockchainConfig []byte,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	for _, node := range network.Nodes {
		if err := TmpNetSetNodeBlockchainConfig(
			networkDir,
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
	networkDir string,
	_ ids.NodeID,
	blockchainID ids.ID,
	blockchainConfig []byte,
) error {
	configPath := filepath.Join(
		networkDir,
		"chains",
		blockchainID.String(),
		"config.json",
	)
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create blockchain config directory %s: %w", configDir, err)
	}
	return os.WriteFile(configPath, blockchainConfig, constants.WriteReadReadPerms)
}

// Set up subnet config for all nodes in the network
func TmpNetSetSubnetConfig(
	networkDir string,
	subnetID ids.ID,
	subnetConfig []byte,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	for _, node := range network.Nodes {
		if err := TmpNetSetNodeSubnetConfig(
			networkDir,
			node.NodeID,
			subnetID,
			subnetConfig,
		); err != nil {
			return err
		}
	}
	return nil
}

// Set up subnet config for a particular node in the network
func TmpNetSetNodeSubnetConfig(
	networkDir string,
	nodeID ids.NodeID,
	subnetID ids.ID,
	subnetConfig []byte,
) error {
	configPath := filepath.Join(
		networkDir,
		nodeID.String(),
		"configs",
		"subnets",
		subnetID.String()+".json",
	)
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create blockchain config directory %s: %w", configDir, err)
	}
	return os.WriteFile(configPath, subnetConfig, constants.WriteReadReadPerms)
}

// Restart all network nodes
// If [subnetIDs] is given, conf the nodes to track the subnets
func TmpNetRestartNodes(
	ctx context.Context,
	log logging.Logger,
	printFunc func(msg string, args ...interface{}),
	networkDir string,
	subnetIDs []ids.ID,
) error {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	for _, node := range network.Nodes {
		if len(subnetIDs) > 0 {
			printFunc("Restarting node %s to track newly deployed subnet/s", node.NodeID)
			subnets, err := node.Flags.GetStringVal(config.TrackSubnetsKey)
			if err != nil {
				return err
			}
			subnets = strings.TrimSpace(subnets)
			subnetsSet := set.Of(strings.Split(subnets, ",")...)
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
	return WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P", ids.Empty)
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
// Add nodes as validators for non [sovereign] flows
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
	// VM Binary setup
	vmID, err := utils.VMID(blockchainName)
	if err != nil {
		return err
	}
	if err := TmpNetInstallVM(networkDir, vmBinaryPath, vmID); err != nil {
		return err
	}
	// Configs
	if blockchainConfig != nil {
		if err := TmpNetSetBlockchainConfig(
			networkDir,
			blockchainID,
			blockchainConfig,
		); err != nil {
			return err
		}
	}
	if subnetConfig != nil {
		if err := TmpNetSetSubnetConfig(
			networkDir,
			subnetID,
			subnetConfig,
		); err != nil {
			return err
		}
	}
	for nodeID, blockchainConfig := range perNodeBlockchainConfig {
		if err := TmpNetSetNodeBlockchainConfig(
			networkDir,
			nodeID,
			blockchainID,
			blockchainConfig,
		); err != nil {
			return err
		}
	}
	// Add subnet to tracked and restart nodes
	if err := TmpNetRestartNodes(
		ctx,
		log,
		printFunc,
		networkDir,
		[]ids.ID{subnetID},
	); err != nil {
		return nil
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P", ids.Empty); err != nil {
		return err
	}
	if !sovereign {
		if err := TmpNetAddNonSovereignValidators(ctx, networkDir, subnetID, wallet); err != nil {
			return err
		}
		if err := TmpNetWaitNonSovereignValidators(ctx, networkDir, subnetID); err != nil {
			return err
		}
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID.String(), subnetID); err != nil {
		return err
	}
	return nil
}

// Add all network nodes of [networkDir] as non SOV validators to [subnetID], using [wallet] to pay for fees
// If a node is already validator for the subnet, does nothing with it
func TmpNetAddNonSovereignValidators(
	ctx context.Context,
	networkDir string,
	subnetID ids.ID,
	wallet *primary.Wallet,
) error {
	endpoint, err := GetTmpNetEndpoint(networkDir)
	if err != nil {
		return err
	}
	pClient := platformvm.NewClient(endpoint)
	vs, err := pClient.GetCurrentValidators(ctx, avagoConstants.PrimaryNetworkID, nil)
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
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
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
func TmpNetWaitNonSovereignValidators(ctx context.Context, networkDir string, subnetID ids.ID) error {
	checkFrequency := time.Second
	endpoint, err := GetTmpNetEndpoint(networkDir)
	if err != nil {
		return err
	}
	pClient := platformvm.NewClient(endpoint)
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
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
	nodeSettings []NodeSettings,
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
			if nodeSettings[i].HTTPPort != 0 {
				node.Flags[config.HTTPPortKey] = nodeSettings[i].HTTPPort
			}
			if nodeSettings[i].P2PPort != 0 {
				node.Flags[config.StakingPortKey] = nodeSettings[i].P2PPort
			}
		}
		if err := node.EnsureKeys(); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
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
	return WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P", ids.Empty)
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

func TmpNetRestartNode(
	ctx context.Context,
	log logging.Logger,
	network *tmpnet.Network,
	node *tmpnet.Node,
) error {
	if err := node.SaveAPIPort(); err != nil {
		return err
	}
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
	_, ok := node.Flags[config.BootstrapIPsKey]
	if !ok {
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

func GetTmpNetNetworkID(networkDir string) (uint32, error) {
	node, err := GetTmpNetFirstNode(networkDir)
	if err != nil {
		return 0, err
	}
	networkIDStr, err := node.Flags.GetStringVal(config.NetworkNameKey)
	if err != nil {
		return 0, err
	}
	networkID, err := strconv.Atoi(networkIDStr)
	if err != nil {
		return 0, err
	}
	return uint32(networkID), nil
}

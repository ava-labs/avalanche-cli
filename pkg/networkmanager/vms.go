// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package networkmanager

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/network"
	"github.com/ava-labs/avalanche-network-runner/pkg/color"
	"github.com/ava-labs/avalanche-network-runner/pkg/logutil"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"go.uber.org/zap"
)

var (
	logLevel       string
	endpoint       string
	dialTimeout    time.Duration
	requestTimeout time.Duration
)

func init() {
	logLevel = logutil.DefaultLogLevel
	endpoint = "0.0.0.0:8080"
	dialTimeout = 10 * time.Second
	requestTimeout = 3 * time.Minute
}

// depends on cli.Health to wait for network readiness check loop
func waitForNetworkReady(ctx context.Context) error {
	cli, err := client.New(client.Config{
		LogLevel:    logLevel,
		Endpoint:    endpoint,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Health(ctx)
	if err != nil {
		return err
	}
	return nil
}

func restartNode(ctx context.Context, nodeName string, whitelistedSubnets string) error {
	cli, err := client.New(client.Config{
		LogLevel:    logLevel,
		Endpoint:    endpoint,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.RestartNode(ctx, nodeName, client.WithWhitelistedSubnets(whitelistedSubnets))
	if err != nil {
		return err
	}
	return nil
}

func checkBlockchain(ctx context.Context, blockchainID ids.ID) error {
	cli, err := client.New(client.Config{
		LogLevel:    logLevel,
		Endpoint:    endpoint,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.CheckBlockchain(ctx, blockchainID.String())
	if err != nil {
		return err
	}
	return nil
}

func getNodeURIs(ctx context.Context) ([]string, error) {
	cli, err := client.New(client.Config{
		LogLevel:    logLevel,
		Endpoint:    endpoint,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	URIs, err := cli.URIs(ctx)
	if err != nil {
		return nil, err
	}
	return URIs, nil
}

func getNodeIDs(ctx context.Context, nodeURIs []string) ([]ids.ShortID, error) {
	nodeIDs := make([]ids.ShortID, 0, len(nodeURIs))
	for i, nodeURI := range nodeURIs {
		infoCli := info.NewClient(nodeURI)
		nodeIDStr, err := infoCli.GetNodeID(ctx)
		if err != nil {
			return nil, err
		}
		nodeIDs[i], err = ids.ShortFromPrefixedString(nodeIDStr, constants.NodeIDPrefix)
		if err != nil {
			return nil, err
		}
	}
	return nodeIDs, nil
}

// provisions local cluster and install custom VMs if applicable
// assumes the local cluster is already set up and healthy
func installVMs(
	ctx context.Context,
	vmNameToGenesis map[string][]byte,
	nwConfig network.Config,
) (map[string]ids.ID, map[string]ids.ID, error) {
	println()
	color.Outf("{{blue}}{{bold}}create and install custom VMs{{/}}\n")

	nodeURIs, err := getNodeURIs(ctx)
	if err != nil {
		return nil, nil, err
	}

	httpRPCEp := nodeURIs[0]
	platformCli := platformvm.NewClient(httpRPCEp)

	baseWallet, avaxAssetID, testKeyAddr, err := setupWallet(ctx, httpRPCEp)
	if err != nil {
		return nil, nil, err
	}
	validatorIDs, err := getNodeIDs(ctx, nodeURIs)
	if err != nil {
		return nil, nil, err
	}
	if err := checkValidators(ctx, platformCli, baseWallet, testKeyAddr, validatorIDs); err != nil {
		return nil, nil, err
	}
	vmNameToSubnetID, err := createSubnets(ctx, baseWallet, testKeyAddr, vmNameToGenesis)
	if err != nil {
		return nil, nil, err
	}
	if err = restartNodesWithWhitelistedSubnets(ctx, vmNameToSubnetID, nwConfig); err != nil {
		return nil, nil, err
	}

	println()
	color.Outf("{{green}}refreshing the wallet with the new URIs after restarts{{/}}\n")
	nodeURIs, err = getNodeURIs(ctx)
	if err != nil {
		return nil, nil, err
	}
	httpRPCEp = nodeURIs[0]
	baseWallet.refresh(httpRPCEp)
	zap.L().Info("set up base wallet with pre-funded test key",
		zap.String("http-rpc-endpoint", httpRPCEp),
		zap.String("address", testKeyAddr.String()),
	)

	if err = addSubnetValidators(ctx, baseWallet, validatorIDs, vmNameToSubnetID); err != nil {
		return nil, nil, err
	}
	vmNameToBlockchainID, err := createBlockchains(ctx, baseWallet, testKeyAddr, vmNameToGenesis, vmNameToSubnetID)
	if err != nil {
		return nil, nil, err
	}

	println()
	color.Outf("{{green}}checking the remaining balance of the base wallet{{/}}\n")
	balances, err := baseWallet.P().Builder().GetBalance()
	if err != nil {
		return nil, nil, err
	}
	zap.L().Info("base wallet AVAX balance",
		zap.String("address", testKeyAddr.String()),
		zap.Uint64("balance", balances[avaxAssetID]),
	)

	return vmNameToSubnetID, vmNameToBlockchainID, nil
}

func waitForVMsReady(ctx context.Context, vmNameToBlockchainID map[string]ids.ID) error {
	nodeURIs, err := getNodeURIs(ctx)
	if err != nil {
		return err
	}
	for vmName := range vmNameToBlockchainID {
		vmID, err := utils.VMID(vmName)
		if err != nil {
			return err
		}
		blockchainID := vmNameToBlockchainID[vmName]
		zap.L().Info("checking blockchain is ready for all",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
			zap.String("blockchain-id", blockchainID.String()),
		)
        if err := checkBlockchain(ctx, blockchainID); err != nil {
            return err
        }
		for _, nodeURI := range nodeURIs {
			color.Outf("{{blue}}{{bold}}[blockchain RPC for %q] \"%s/ext/bc/%s\"{{/}}\n", vmID, nodeURI, blockchainID.String())
		}
	}
	return nil
}

func setupWallet(ctx context.Context, httpRPCEp string) (baseWallet *refreshableWallet, avaxAssetID ids.ID, testKeyAddr ids.ShortID, err error) {
	// "local/default/genesis.json" pre-funds "ewoq" key
	testKey := genesis.EWOQKey
	testKeyAddr = testKey.PublicKey().Address()
	testKeychain := secp256k1fx.NewKeychain(genesis.EWOQKey)

	println()
	color.Outf("{{green}}setting up the base wallet with the seed test key{{/}}\n")
	baseWallet, err = createRefreshableWallet(ctx, httpRPCEp, testKeychain)
	if err != nil {
		return nil, ids.Empty, ids.ShortEmpty, err
	}
	zap.L().Info("set up base wallet with pre-funded test key",
		zap.String("http-rpc-endpoint", httpRPCEp),
		zap.String("address", testKeyAddr.String()),
	)

	println()
	color.Outf("{{green}}check if the seed test key has enough balance to create validators and subnets{{/}}\n")
	avaxAssetID = baseWallet.P().AVAXAssetID()
	balances, err := baseWallet.P().Builder().GetBalance()
	if err != nil {
		return nil, ids.Empty, ids.ShortEmpty, err
	}
	bal, ok := balances[avaxAssetID]
	if bal <= 1*units.Avax || !ok {
		return nil, ids.Empty, ids.ShortEmpty, fmt.Errorf("not enough AVAX balance %v in the address %q", bal, testKeyAddr)
	}
	zap.L().Info("fetched base wallet AVAX balance",
		zap.String("http-rpc-endpoint", httpRPCEp),
		zap.String("address", testKeyAddr.String()),
		zap.Uint64("balance", bal),
	)

	return baseWallet, avaxAssetID, testKeyAddr, nil
}

func checkValidators(
	ctx context.Context,
	platformCli platformvm.Client,
	baseWallet *refreshableWallet,
	testKeyAddr ids.ShortID,
	validatorIDs []ids.ShortID,
) error {
	println()
	color.Outf("{{green}}fetching all nodes from the existing cluster to make sure all nodes are validating the primary network/subnet{{/}}\n")
	// ref. https://docs.avax.network/build/avalanchego-apis/p-chain/#platformgetcurrentvalidators
	cctx, cancel := createDefaultCtx(ctx)
	vs, err := platformCli.GetCurrentValidators(cctx, constants.PrimaryNetworkID, nil)
	cancel()
	if err != nil {
		return err
	}
	curValidators := make(map[ids.ShortID]struct{})
	for _, v := range vs {
		va, ok := v.(map[string]interface{})
		if !ok {
			return fmt.Errorf("failed to parse validator data: %T %+v", v, v)
		}
		nodeIDStr, ok := va["nodeID"].(string)
		if !ok {
			return fmt.Errorf("failed to parse validator data: %T %+v", va, va)
		}
		nodeID, err := ids.ShortFromPrefixedString(nodeIDStr, constants.NodeIDPrefix)
		if err != nil {
			return err
		}
		curValidators[nodeID] = struct{}{}
		zap.L().Info("current validator", zap.String("node-id", nodeID.String()))
	}

	println()
	color.Outf("{{green}}adding all nodes as validator for the primary subnet{{/}}\n")
	for _, nodeID := range validatorIDs {
		_, isValidator := curValidators[nodeID]
		if isValidator {
			zap.L().Info("the node is already validating the primary subnet; skipping",
				zap.String("node-id", nodeID.String()),
			)
			continue
		}

		zap.L().Info("adding a node as a validator to the primary subnet",
			zap.String("node-id", nodeID.String()),
		)
		cctx, cancel = createDefaultCtx(ctx)
		txID, err := baseWallet.P().IssueAddValidatorTx(
			&platformvm.Validator{
				NodeID: nodeID,
				Start:  uint64(time.Now().Add(10 * time.Second).Unix()),
				End:    uint64(time.Now().Add(300 * time.Hour).Unix()),
				Wght:   1 * units.Avax,
			},
			&secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{testKeyAddr},
			},
			10*10000, // 10% fee percent, times 10000 to make it as shares
			common.WithContext(cctx),
			defaultPoll,
		)
		cancel()
		if err != nil {
			return err
		}
		zap.L().Info("added the node as primary subnet validator",
			zap.String("node-id", nodeID.String()),
			zap.String("tx-id", txID.String()),
		)
	}
	return nil
}

func createSubnets(
	ctx context.Context,
	baseWallet *refreshableWallet,
	testKeyAddr ids.ShortID,
	vmNameToGenesis map[string][]byte,
) (map[string]ids.ID, error) {
	println()
	color.Outf("{{green}}creating subnet for each custom VM{{/}}\n")
	vmNameToSubnetID := map[string]ids.ID{}
	for vmName := range vmNameToGenesis {
		vmID, err := utils.VMID(vmName)
		if err != nil {
			return nil, err
		}
		zap.L().Info("creating subnet tx",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
		)
		cctx, cancel := createDefaultCtx(ctx)
		subnetID, err := baseWallet.P().IssueCreateSubnetTx(
			&secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{testKeyAddr},
			},
			common.WithContext(cctx),
			defaultPoll,
		)
		cancel()
		if err != nil {
			return nil, err
		}
		zap.L().Info("created subnet tx",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
			zap.String("subnet-id", subnetID.String()),
		)
		vmNameToSubnetID[vmName] = subnetID
	}
	return vmNameToSubnetID, nil
}

// TODO: make this "restart" pattern more generic, so it can be used for "Restart" RPC
func restartNodesWithWhitelistedSubnets(
	ctx context.Context,
	vmNameToSubnetID map[string]ids.ID,
	nwConfig network.Config,
) (err error) {
	println()
	color.Outf("{{green}}restarting each node with --whitelisted-subnets{{/}}\n")
	whitelistedSubnetIDs := make([]string, 0, len(vmNameToSubnetID))
	for _, subnetID := range vmNameToSubnetID {
		whitelistedSubnetIDs = append(whitelistedSubnetIDs, subnetID.String())
	}
	sort.Strings(whitelistedSubnetIDs)
	whitelistedSubnets := strings.Join(whitelistedSubnetIDs, ",")
	for i := range nwConfig.NodeConfigs {
		nodeName := nwConfig.NodeConfigs[i].Name

		zap.L().Info("updating node config and info",
			zap.String("node-name", nodeName),
			zap.String("whitelisted-subnets", whitelistedSubnets),
		)

		// replace "whitelisted-subnets" flag
		nwConfig.NodeConfigs[i].ConfigFile, err = utils.UpdateJSONKey(nwConfig.NodeConfigs[i].ConfigFile, "whitelisted-subnets", whitelistedSubnets)
		if err != nil {
			return err
		}
	}
	zap.L().Info("restarting all nodes to whitelist subnet",
		zap.Strings("whitelisted-subnets", whitelistedSubnetIDs),
	)
	for _, nodeConfig := range nwConfig.NodeConfigs {
		nodeName := nodeConfig.Name

		zap.L().Info("removing and adding back the node for whitelisted subnets", zap.String("node-name", nodeName))
		if err := restartNode(ctx, nodeName, whitelistedSubnets); err != nil {
			return err
		}
		zap.L().Info("waiting for local cluster readiness after restart", zap.String("node-name", nodeName))
		if err := waitForNetworkReady(ctx); err != nil {
			return err
		}
	}
	return nil
}

func addSubnetValidators(
	ctx context.Context,
	baseWallet *refreshableWallet,
	validatorIDs []ids.ShortID,
	vmNameToSubnetID map[string]ids.ID,
) error {
	println()
	color.Outf("{{green}}adding all nodes as subnet validator for each subnet{{/}}\n")
	for vmName := range vmNameToSubnetID {
		vmID, err := utils.VMID(vmName)
		if err != nil {
			return err
		}
		subnetID := vmNameToSubnetID[vmName]
		zap.L().Info("adding all nodes as subnet validator",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
			zap.String("subnet-id", subnetID.String()),
		)
		for _, validatorID := range validatorIDs {
			cctx, cancel := createDefaultCtx(ctx)
			txID, err := baseWallet.P().IssueAddSubnetValidatorTx(
				&platformvm.SubnetValidator{
					Validator: platformvm.Validator{
						NodeID: validatorID,

						// reasonable delay in most/slow test environments
						Start: uint64(time.Now().Add(time.Minute).Unix()),
						End:   uint64(time.Now().Add(100 * time.Hour).Unix()),
						Wght:  1000,
					},
					Subnet: subnetID,
				},
				common.WithContext(cctx),
				defaultPoll,
			)
			cancel()
			if err != nil {
				return err
			}
			zap.L().Info("added the node as a subnet validator",
				zap.String("vm-name", vmName),
				zap.String("vm-id", vmID.String()),
				zap.String("subnet-id", subnetID.String()),
				zap.String("node-id", validatorID.String()),
				zap.String("tx-id", txID.String()),
			)
		}
	}
	return nil
}

func createBlockchains(
	ctx context.Context,
	baseWallet *refreshableWallet,
	testKeyAddr ids.ShortID,
	vmNameToGenesis map[string][]byte,
	vmNameToSubnetID map[string]ids.ID,
) (map[string]ids.ID, error) {
	println()
	color.Outf("{{green}}creating blockchain for each custom VM{{/}}\n")
	vmNameToBlockchainID := map[string]ids.ID{}
	for vmName := range vmNameToSubnetID {
		vmID, err := utils.VMID(vmName)
		if err != nil {
			return nil, err
		}
		subnetID := vmNameToSubnetID[vmName]
		vmGenesisBytes := vmNameToGenesis[vmName]

		zap.L().Info("creating blockchain tx",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
			zap.Int("genesis-bytes", len(vmGenesisBytes)),
		)
		cctx, cancel := createDefaultCtx(ctx)
		blockchainID, err := baseWallet.P().IssueCreateChainTx(
			subnetID,
			vmGenesisBytes,
			vmID,
			nil,
			vmName,
			common.WithContext(cctx),
			defaultPoll,
		)
		cancel()
		if err != nil {
			return nil, err
		}

		vmNameToBlockchainID[vmName] = blockchainID
		zap.L().Info("created a new blockchain",
			zap.String("vm-name", vmName),
			zap.String("vm-id", vmID.String()),
			zap.String("blockchain-id", blockchainID.String()),
		)
	}
	return vmNameToBlockchainID, nil
}

var defaultPoll = common.WithPollFrequency(5 * time.Second)

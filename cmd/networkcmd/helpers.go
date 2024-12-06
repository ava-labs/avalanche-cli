// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoConstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

func TrackSubnet(
	blockchainName string,
	avalancheGoBinPath string,
	sovereign bool,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	network := models.NewLocalNetwork()
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, network.Name())
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	blockchainID := sc.Networks[network.Name()].BlockchainID
	vmID, err := anrutils.VMID(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	var vmBin string
	switch sc.VM {
	case models.SubnetEvm:
		_, vmBin, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.CustomVM:
		vmBin = binutils.SetupCustomBin(app, blockchainName)
	default:
		return fmt.Errorf("unknown vm: %s", sc.VM)
	}

	pluginPath := filepath.Join(app.GetPluginsDir(), vmID.String())
	if err := utils.FileCopy(vmBin, pluginPath); err != nil {
		return err
	}
	if err := os.Chmod(pluginPath, constants.DefaultPerms755); err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalNetworkGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	publicEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if app.ChainConfigExists(blockchainName) {
			inputChainConfigPath := app.GetChainConfigPath(blockchainName)
			outputChainConfigPath := filepath.Join(
				status.ClusterInfo.RootDataDir,
				nodeInfo.Name,
				"configs",
				"chains",
				blockchainID.String(),
				"config.json",
			)
			if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
				return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
			}
			if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
				return err
			}
		}
		if app.AvagoSubnetConfigExists(blockchainName) {
			inputSubnetConfigPath := app.GetAvagoSubnetConfigPath(blockchainName)
			outputSubnetConfigPath := filepath.Join(
				status.ClusterInfo.RootDataDir,
				nodeInfo.Name,
				"configs",
				"subnets",
				subnetID.String()+".json",
			)
			if err := os.MkdirAll(filepath.Dir(outputSubnetConfigPath), 0o700); err != nil {
				return fmt.Errorf("could not create subnet conf directory %s: %w", filepath.Dir(outputSubnetConfigPath), err)
			}
			if err := utils.FileCopy(inputSubnetConfigPath, outputSubnetConfigPath); err != nil {
				return err
			}
		}
		perNodeChainConfigPath := filepath.Join(app.GetSubnetDir(), blockchainName, constants.PerNodeChainConfigFileName)
		if utils.FileExists(perNodeChainConfigPath) {
			perNodeChainConfig, err := utils.ReadJSON(perNodeChainConfigPath)
			if err != nil {
				return err
			}
			for nodeName, cfg := range perNodeChainConfig {
				cfgBytes, err := json.Marshal(cfg)
				if err != nil {
					return err
				}
				outputChainConfigPath := filepath.Join(
					status.ClusterInfo.RootDataDir,
					nodeName,
					"configs",
					"chains",
					blockchainID.String(),
					"config.json",
				)
				if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
					return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
				}
				if err := os.WriteFile(outputChainConfigPath, cfgBytes, constants.WriteReadReadPerms); err != nil {
					return err
				}
			}
		}
		ux.Logger.PrintToUser("Restarting node %s to track subnet", nodeInfo.Name)
		subnets := strings.TrimSpace(nodeInfo.WhitelistedSubnets)
		if subnets != "" {
			subnets += ","
		}
		subnets += subnetID.String()
		opts := []client.OpOption{
			client.WithWhitelistedSubnets(subnets),
			client.WithExecPath(avalancheGoBinPath),
		}
		if _, err := cli.RestartNode(ctx, nodeInfo.Name, opts...); err != nil {
			return err
		}
		publicEndpoints = append(publicEndpoints, nodeInfo.Uri)
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(models.GetRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(models.GetWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	for _, rpcURL := range networkInfo.RPCEndpoints {
		ux.Logger.PrintToUser("Waiting for %s to be available", rpcURL)
		if err := evm.WaitForRPC(ctx, rpcURL); err != nil {
			return err
		}
	}
	if err := IsBootstrapped(cli, blockchainID.String()); err != nil {
		return err
	}
	if err := IsBootstrapped(cli, "P"); err != nil {
		return err
	}
	if _, err := cli.UpdateStatus(ctx); err != nil {
		return err
	}
	if err := SetAlias(cli, blockchainID.String(), blockchainName); err != nil {
		return err
	}
	if !sovereign {
		if err := AddNoSovereignValidators(cli, subnetID); err != nil {
			return err
		}
		if err := WaitNoSovereignValidators(cli, subnetID); err != nil {
			return err
		}
	}
	sc.Networks[network.Name()] = networkInfo
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", network.Name(), blockchainName)
	return nil
}

func IsBootstrapped(cli client.Client, blockchainID string) error {
	blockchainBootstrapCheckFrequency := time.Second
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		for {
			infoClient := info.NewClient(nodeInfo.GetUri())
			boostrapped, err := infoClient.IsBootstrapped(ctx, blockchainID)
			if err != nil && !strings.Contains(err.Error(), "there is no chain with alias/ID") {
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
	}
	return err
}

func SetAlias(cli client.Client, blockchainID string, alias string) error {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		adminClient := admin.NewClient(nodeInfo.GetUri())
		if err := adminClient.AliasChain(ctx, blockchainID, alias); err != nil {
			return err
		}
	}
	return nil
}

func AddNoSovereignValidators(cli client.Client, subnetID ids.ID) error {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	nodeInfo, ok := status.ClusterInfo.NodeInfos["node1"]
	if !ok {
		return fmt.Errorf("node1 not found on local network")
	}
	pClient := platformvm.NewClient(nodeInfo.GetUri())
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
	k, err := app.GetKey("ewoq", models.NewLocalNetwork(), false)
	if err != nil {
		return err
	}
	wallet, err := primary.MakeWallet(
		ctx,
		constants.LocalAPIEndpoint,
		k.KeyChain(),
		secp256k1fx.NewKeychain(),
		primary.WalletConfig{
			SubnetIDs: []ids.ID{subnetID},
		},
	)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		nodeIDStr := nodeInfo.GetId()
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
		if isValidator := subnetValidators.Contains(nodeID); isValidator {
			continue
		}
		if _, err := wallet.P().IssueAddSubnetValidatorTx(
			&txs.SubnetValidator{
				Validator: txs.Validator{
					NodeID: nodeID,
					End:    uint64(primaryValidatorsEndtime[nodeID].Unix()),
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

func WaitNoSovereignValidators(cli client.Client, subnetID ids.ID) error {
	checkFrequency := time.Second
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	nodeInfo, ok := status.ClusterInfo.NodeInfos["node1"]
	if !ok {
		return fmt.Errorf("node1 not found on local network")
	}
	pClient := platformvm.NewClient(nodeInfo.GetUri())
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		for {
			vs, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
			if err != nil {
				return err
			}
			subnetValidators := set.Set[ids.NodeID]{}
			for _, v := range vs {
				subnetValidators.Add(v.NodeID)
			}
			nodeIDStr := nodeInfo.GetId()
			nodeID, err := ids.NodeIDFromString(nodeIDStr)
			if err != nil {
				return err
			}
			if subnetValidators.Contains(nodeID) {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(checkFrequency):
			}
		}
	}
	return err
}

func AlreadyDeployed(blockchainName string) (bool, error) {
	chainVMID, err := anrutils.VMID(blockchainName)
	if err != nil {
		return false, fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalNetworkGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return false, err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return false, err
	}
	if status.ClusterInfo != nil {
		for _, chainInfo := range status.ClusterInfo.CustomChains {
			if chainInfo.VmId == chainVMID.String() {
				return true, nil
			}
		}
	}
	return false, nil
}

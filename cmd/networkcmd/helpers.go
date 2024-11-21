// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
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

func determineAvagoVersion(userProvidedAvagoVersion string) (string, error) {
	// a specific user provided version should override this calculation, so just return
	if userProvidedAvagoVersion != latest {
		return userProvidedAvagoVersion, nil
	}

	// Need to determine which subnets have been deployed
	locallyDeployedSubnets, err := subnet.GetLocallyDeployedSubnetsFromFile(app)
	if err != nil {
		return "", err
	}

	// if no subnets have been deployed, use latest
	if len(locallyDeployedSubnets) == 0 {
		return latest, nil
	}

	currentRPCVersion := -1

	// For each deployed subnet, check RPC versions
	for _, deployedSubnet := range locallyDeployedSubnets {
		sc, err := app.LoadSidecar(deployedSubnet)
		if err != nil {
			return "", err
		}

		// if you have a custom vm, you must provide the version explicitly
		// if you upgrade from subnet-evm to a custom vm, the RPC version will be 0
		if sc.VM == models.CustomVM || sc.Networks[models.Local.String()].RPCVersion == 0 {
			continue
		}

		if currentRPCVersion == -1 {
			currentRPCVersion = sc.Networks[models.Local.String()].RPCVersion
		}

		if sc.Networks[models.Local.String()].RPCVersion != currentRPCVersion {
			return "", fmt.Errorf(
				"RPC version mismatch. Expected %d, got %d for Subnet %s. Upgrade all subnets to the same RPC version to launch the network",
				currentRPCVersion,
				sc.RPCVersion,
				sc.Name,
			)
		}
	}

	// If currentRPCVersion == -1, then only custom subnets have been deployed, the user must provide the version explicitly if not latest
	if currentRPCVersion == -1 {
		ux.Logger.PrintToUser("No Subnet RPC version found. Using latest AvalancheGo version")
		return latest, nil
	}

	return vm.GetLatestAvalancheGoByProtocolVersion(
		app,
		currentRPCVersion,
		constants.AvalancheGoCompatibilityURL,
	)
}

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
		ux.Logger.PrintToUser("Restarting node %s to track subnet", nodeInfo.Name)
		opts := []client.OpOption{
			client.WithWhitelistedSubnets(subnetID.String()),
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
		ux.Logger.PrintToUser("Waiting for rpc %s to be available", rpcURL)
		if err := evm.WaitForRPC(ctx, rpcURL); err != nil {
			return err
		}
	}
	if _, err := cli.WaitForHealthy(ctx); err != nil {
		return err
	}
	if err := IsBootstrapped(cli, blockchainID.String()); err != nil {
		return err
	}
	if err := IsBootstrapped(cli, "P"); err != nil {
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
		&primary.WalletConfig{
			URI:          constants.LocalAPIEndpoint,
			AVAXKeychain: k.KeyChain(),
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    []ids.ID{subnetID},
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

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	avagoConstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

func LocalNetworkTrackSubnet(
	ctx context.Context,
	app *application.Avalanche,
	blockchainName string,
	avalancheGoBinPath string,
) error {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return err
	}
	var (
		blockchainConfig []byte
		subnetConfig     []byte
	)
	if app.ChainConfigExists(blockchainName) {
		blockchainConfig, err = os.ReadFile(app.GetChainConfigPath(blockchainName))
		if err != nil {
			return err
		}
	}
	if app.AvagoSubnetConfigExists(blockchainName) {
		subnetConfig, err = os.ReadFile(app.GetAvagoSubnetConfigPath(blockchainName))
		if err != nil {
			return err
		}
	}
	perNodeBlockchainConfig, err := GetPerNodeBlockchainConfig(app, blockchainName)
	if err != nil {
		return err
	}
	return TmpNetTrackSubnet(
		ctx,
		app,
		networkDir,
		blockchainName,
		avalancheGoBinPath,
		blockchainConfig,
		subnetConfig,
		perNodeBlockchainConfig,
	)
}

func TmpNetTrackSubnet(
	ctx context.Context,
	app *application.Avalanche,
	networkDir string,
	blockchainName string,
	avalancheGoBinPath string,
	blockchainConfig []byte,
	subnetConfig []byte,
	perNodeBlockchainConfig map[ids.NodeID][]byte,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	sovereign := sc.Sovereign
	networkName := models.NewLocalNetwork().Name()
	if sc.Networks[networkName].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, networkName)
	}
	subnetID := sc.Networks[networkName].SubnetID
	blockchainID := sc.Networks[networkName].BlockchainID

	// VM binary setup
	vmID, err := utils.VMID(blockchainName)
	if err != nil {
		return err
	}
	binaryPath, err := SetupVMBinary(app, blockchainName)
	if err != nil {
		return fmt.Errorf("failed to setup VM binary: %w", err)
	}
	if err := TmpNetInstallVM(networkDir, binaryPath, vmID); err != nil {
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
		ux.Logger.PrintToUser("Restarting node %s to track newly deployed network", nodeInfo.Name)
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

	networkInfo := sc.Networks[networkName]
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
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID.String()); err != nil {
		return err
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P"); err != nil {
		return err
	}
	if _, err := cli.UpdateStatus(ctx); err != nil {
		return err
	}
	if err := TmpNetSetAlias(networkDir, blockchainID.String(), blockchainName); err != nil {
		return err
	}
	if !sovereign {
		if err := AddNoSovereignValidators(app, cli, subnetID); err != nil {
			return err
		}
		if err := WaitNoSovereignValidators(cli, subnetID); err != nil {
			return err
		}
	}
	sc.Networks[networkName] = networkInfo
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", networkName, blockchainName)
	return nil
}

func AddNoSovereignValidators(
	app *application.Avalanche,
	cli client.Client,
	subnetID ids.ID,
) error {
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

func BlockchainAlreadyDeployedOnLocalNetwork(app *application.Avalanche, blockchainName string) (bool, error) {
	chainVMID, err := utils.VMID(blockchainName)
	if err != nil {
		return false, fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	blockchains, err := GetLocalNetworkBlockchainInfo(app)
	if err != nil {
		return false, err
	}
	for _, chain := range blockchains {
		if chain.VMID == chainVMID {
			return true, nil
		}
	}
	return false, nil
}

func GetLocalNetworkRelayerConfigPath(app *application.Avalanche, networkDir string) (bool, string, error) {
	if networkDir == "" {
		var err error
		networkDir, err = GetLocalNetworkDir(app)
		if err != nil {
			return false, "", err
		}
	}
	relayerConfigPath := app.GetLocalRelayerConfigPath(models.Local, networkDir)
	return utils.FileExists(relayerConfigPath), relayerConfigPath, nil
}

func GetDefaultContext() (context.Context, context.CancelFunc) {
	return utils.GetTimedContext(2 * time.Minute)
}

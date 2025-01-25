// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	avagoConstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
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
	avalancheGoBinaryPath string,
) error {
	networkModel := models.NewLocalNetwork()
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return err
	}
	var (
		blockchainConfig []byte
		subnetConfig     []byte
	)
	vmBinaryPath, err := SetupVMBinary(app, blockchainName)
	if err != nil {
		return fmt.Errorf("failed to setup VM binary: %w", err)
	}
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
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[networkModel.Name()].SubnetID
	wallet, err := GetLocalNetworkWallet(ctx, app, []ids.ID{subnetID})
	if err != nil {
		return err
	}
	if err := TmpNetTrackSubnet(
		ctx,
		app,
		networkModel,
		networkDir,
		blockchainName,
		avalancheGoBinaryPath,
		vmBinaryPath,
		blockchainConfig,
		subnetConfig,
		perNodeBlockchainConfig,
		wallet,
	); err != nil {
		return err
	}
	nodeURIs, err := GetTmpNetNodeURIs(networkDir)
	if err != nil {
		return err
	}
	return PersistDefaultBlockchainEndpoints(
		app,
		networkModel,
		nodeURIs,
		blockchainName,
	)
}

func TmpNetTrackSubnet(
	ctx context.Context,
	app *application.Avalanche,
	networkModel models.Network,
	networkDir string,
	blockchainName string,
	avalancheGoBinaryPath string,
	vmBinaryPath string,
	blockchainConfig []byte,
	subnetConfig []byte,
	perNodeBlockchainConfig map[ids.NodeID][]byte,
	wallet *primary.Wallet,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	sovereign := sc.Sovereign
	if sc.Networks[networkModel.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, networkModel.Name())
	}
	blockchainID := sc.Networks[networkModel.Name()].BlockchainID
	subnetID := sc.Networks[networkModel.Name()].SubnetID

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
	if err := TmpNetRestartNodesToTrackSubnet(
		ctx,
		app.Log,
		ux.Logger.PrintToUser,
		networkDir,
		subnetID,
	); err != nil {
		return nil
	}

	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID.String()); err != nil {
		return err
	}
	if err := WaitTmpNetBlockchainBootstrapped(ctx, networkDir, "P"); err != nil {
		return err
	}
	if err := TmpNetSetAlias(networkDir, blockchainID.String(), blockchainName); err != nil {
		return err
	}
	if !sovereign {
		if err := TmpNetAddNonSovereignValidators(ctx, app, networkDir, subnetID, wallet); err != nil {
			return err
		}
		if err := TmpNetWaitNonSovereignValidators(ctx, networkDir, subnetID); err != nil {
			return err
		}
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", networkModel.Name(), blockchainName)
	return nil
}

func TmpNetAddNonSovereignValidators(
	ctx context.Context,
	app *application.Avalanche,
	networkDir string,
	subnetID ids.ID,
	wallet *primary.Wallet,
) error {
	endpoint, err := GetTmpNetworkEndpoint(networkDir)
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
	network, err := tmpnet.ReadNetwork(networkDir)
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

func TmpNetWaitNonSovereignValidators(ctx context.Context, networkDir string, subnetID ids.ID) error {
	checkFrequency := time.Second
	endpoint, err := GetTmpNetworkEndpoint(networkDir)
	if err != nil {
		return err
	}
	pClient := platformvm.NewClient(endpoint)
	network, err := tmpnet.ReadNetwork(networkDir)
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

func GetLocalNetworkWallet(
	ctx context.Context,
	app *application.Avalanche,
	subnetIDs []ids.ID,
) (*primary.Wallet, error) {
	endpoint, err := GetLocalNetworkEndpoint(app)
	if err != nil {
		return nil, err
	}
	ewoqKey, err := app.GetKey("ewoq", models.NewLocalNetwork(), false)
	if err != nil {
		return nil, err
	}
	return primary.MakeWallet(
		ctx,
		endpoint,
		ewoqKey.KeyChain(),
		secp256k1fx.NewKeychain(),
		primary.WalletConfig{
			SubnetIDs: subnetIDs,
		},
	)
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
)

type LocalDeployer struct {
	binChecker             binutils.BinaryChecker
	getClientFunc          getGRPCClientFunc
	binaryDownloader       binutils.PluginBinaryDownloader
	app                    *application.Avalanche
	avagoVersion           string
	avagoBinaryPath        string
	vmBin                  string
	installSnapshotUpdates bool
}

// uses either avagoVersion or avagoBinaryPath
func NewLocalDeployer(
	app *application.Avalanche,
	avagoVersion string,
	avagoBinaryPath string,
	vmBin string,
	installSnapshotUpdates bool,
) *LocalDeployer {
	return &LocalDeployer{
		binChecker:             binutils.NewBinaryChecker(),
		getClientFunc:          binutils.NewGRPCClient,
		binaryDownloader:       binutils.NewPluginBinaryDownloader(app),
		app:                    app,
		avagoVersion:           avagoVersion,
		avagoBinaryPath:        avagoBinaryPath,
		vmBin:                  vmBin,
		installSnapshotUpdates: installSnapshotUpdates,
	}
}

type getGRPCClientFunc func(...binutils.GRPCClientOpOption) (client.Client, error)

type ICMSpec struct {
	SkipICMDeploy                bool
	SkipRelayerDeploy            bool
	ICMVersion                   string
	MessengerContractAddressPath string
	MessengerDeployerAddressPath string
	MessengerDeployerTxPath      string
	RegistryBydecodePath         string
	RelayerVersion               string
	RelayerBinPath               string
	RelayerLogLevel              string
}

type DeployInfo struct {
	SubnetID            ids.ID
	BlockchainID        ids.ID
	ICMMessengerAddress string
	ICMRegistryAddress  string
}

// SetupLocalEnv also does some heavy lifting:
// * sets up default snapshot if not installed
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *LocalDeployer) SetupLocalEnv() (string, error) {
	avalancheGoBinPath := ""
	if d.avagoBinaryPath != "" {
		avalancheGoBinPath = d.avagoBinaryPath
	} else {
		var (
			avagoDir string
			err      error
		)
		_, avagoDir, err = d.setupLocalEnv()
		if err != nil {
			return "", fmt.Errorf("failed setting up local environment: %w", err)
		}
		avalancheGoBinPath = filepath.Join(avagoDir, "avalanchego")
	}

	pluginDir := d.app.GetPluginsDir()

	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return "", fmt.Errorf("could not create pluginDir %s", pluginDir)
	}

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return "", fmt.Errorf(
			"evaluated avalancheGoBinPath to be %s but it does not exist", avalancheGoBinPath)
	}

	return avalancheGoBinPath, nil
}

func (d *LocalDeployer) setupLocalEnv() (string, string, error) {
	return binutils.SetupAvalanchego(d.app, d.avagoVersion)
}

// WaitForHealthy polls continuously until the network is ready to be used
func WaitForHealthy(
	ctx context.Context,
	cli client.Client,
) (*rpcpb.ClusterInfo, error) {
	cancel := make(chan struct{})
	defer close(cancel)
	go ux.PrintWait(cancel)
	resp, err := cli.WaitForHealthy(ctx)
	if err != nil {
		return nil, err
	}
	return resp.ClusterInfo, nil
}

// HasEndpoints returns true if cluster info contains custom blockchains
func HasEndpoints(clusterInfo *rpcpb.ClusterInfo) bool {
	return len(clusterInfo.CustomChains) > 0
}

// Returns an error if the server cannot be contacted. You may want to ignore this error.
func GetLocallyDeployedSubnets() (map[string]struct{}, error) {
	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}

	for _, chain := range resp.GetClusterInfo().CustomChains {
		deployedNames[chain.ChainName] = struct{}{}
	}

	return deployedNames, nil
}

func IssueRemoveSubnetValidatorTx(kc keychain.Keychain, subnetID ids.ID, nodeID ids.NodeID) (ids.ID, error) {
	ctx := context.Background()
	api := constants.LocalAPIEndpoint
	wallet, err := primary.MakeWallet(
		ctx,
		api,
		kc,
		secp256k1fx.NewKeychain(),
		primary.WalletConfig{
			SubnetIDs: []ids.ID{subnetID},
		},
	)
	if err != nil {
		return ids.Empty, err
	}

	tx, err := wallet.P().IssueRemoveSubnetValidatorTx(nodeID, subnetID)
	return tx.ID(), err
}

func GetSubnetValidators(subnetID ids.ID) ([]platformvm.ClientPermissionlessValidator, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	return pClient.GetCurrentValidators(ctx, subnetID, nil)
}

func CheckNodeIsInSubnetValidators(subnetID ids.ID, nodeID string) (bool, error) {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v.NodeID.String() == nodeID {
			return true, nil
		}
	}
	return false, nil
}

func GetCurrentSupply(subnetID ids.ID) error {
	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	_, _, err := pClient.GetCurrentSupply(ctx, subnetID)
	return err
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
)

// information that is persisted alongside the local network
type ExtraLocalNetworkData struct {
	RelayerPath                      string
	CChainTeleporterMessengerAddress string
	CChainTeleporterRegistryAddress  string
}

// Restart all nodes on local network to track [blockchainName].
// Before that, set up VM binary, blockchain and subnet config information
// After the blockchain is bootstrapped, add alias for [blockchainName]->[blockchainID]
// Finally persist all new blockchain RPC URLs into blockchain sidecar.
func LocalNetworkTrackSubnet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	blockchainName string,
) error {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return err
	}
	networkModel := models.NewLocalNetwork()
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	if sc.Networks[networkModel.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, networkModel.Name())
	}
	subnetID := sc.Networks[networkModel.Name()].SubnetID
	wallet, err := GetLocalNetworkWallet(app, []ids.ID{subnetID})
	if err != nil {
		return err
	}
	return TrackSubnet(
		app,
		printFunc,
		blockchainName,
		networkDir,
		networkModel,
		wallet,
	)
}

// Indicates if [blockchainName] is found to be deployed on the local network, based on the VMID associated to it
func BlockchainAlreadyDeployedOnLocalNetwork(app *application.Avalanche, blockchainName string) (bool, error) {
	chainVMID, err := utils.VMID(blockchainName)
	if err != nil {
		return false, fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	blockchains, err := GetLocalNetworkBlockchainsInfo(app)
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

// Returns the configuration file for the local network relayer
// if [networkDir] is given, assumes that the local network is running from that dir
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

// GetLocalNetworkWallet returns a wallet that can operate on the local network
// initialized to recognice all given [subnetIDs] as pre generated
func GetLocalNetworkWallet(
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
	ctx, cancel := sdkutils.GetTimedContext(constants.WalletCreationTimeout)
	defer cancel()
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

// Gathers extra information for the local network, not available on the primary storage
func GetExtraLocalNetworkData(app *application.Avalanche, rootDataDir string) (bool, ExtraLocalNetworkData, error) {
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if rootDataDir == "" {
		var err error
		rootDataDir, err = GetLocalNetworkDir(app)
		if err != nil {
			return false, extraLocalNetworkData, err
		}
	}
	extraLocalNetworkDataPath := filepath.Join(rootDataDir, constants.ExtraLocalNetworkDataFilename)
	if !utils.FileExists(extraLocalNetworkDataPath) {
		return false, extraLocalNetworkData, nil
	}
	bs, err := os.ReadFile(extraLocalNetworkDataPath)
	if err != nil {
		return false, extraLocalNetworkData, err
	}
	if err := json.Unmarshal(bs, &extraLocalNetworkData); err != nil {
		return false, extraLocalNetworkData, err
	}
	return true, extraLocalNetworkData, nil
}

// Writes extra information for the local network, not available on the primary storage
func WriteExtraLocalNetworkData(
	app *application.Avalanche,
	rootDataDir string,
	relayerPath string,
	cchainICMMessengerAddress string,
	cchainICMRegistryAddress string,
) error {
	if rootDataDir == "" {
		var err error
		rootDataDir, err = GetLocalNetworkDir(app)
		if err != nil {
			return err
		}
	}
	extraLocalNetworkData := ExtraLocalNetworkData{}
	extraLocalNetworkDataPath := filepath.Join(rootDataDir, constants.ExtraLocalNetworkDataFilename)
	if utils.FileExists(extraLocalNetworkDataPath) {
		var err error
		_, extraLocalNetworkData, err = GetExtraLocalNetworkData(app, rootDataDir)
		if err != nil {
			return err
		}
	}
	if relayerPath != "" {
		extraLocalNetworkData.RelayerPath = utils.ExpandHome(relayerPath)
	}
	if cchainICMMessengerAddress != "" {
		extraLocalNetworkData.CChainTeleporterMessengerAddress = cchainICMMessengerAddress
	}
	if cchainICMRegistryAddress != "" {
		extraLocalNetworkData.CChainTeleporterRegistryAddress = cchainICMRegistryAddress
	}
	bs, err := json.Marshal(&extraLocalNetworkData)
	if err != nil {
		return err
	}
	return os.WriteFile(extraLocalNetworkDataPath, bs, constants.WriteReadReadPerms)
}

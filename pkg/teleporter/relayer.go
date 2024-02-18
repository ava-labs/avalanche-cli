// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/awm-relayer/config"
	offchainregistry "github.com/ava-labs/awm-relayer/messages/off-chain-registry"
)

const (
	teleporterRelayerPrivateKey = "C2CE4E001B7585F543982A01FBC537CFF261A672FA8BD1FAFC08A207098FE2DE"
	teleporterRelayerAddress    = "0xA100fF48a37cab9f87c8b5Da933DA46ea1a5fb80"
)

var (
	teleporterRelayerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX
)

func FundRelayer(
	rpcURL string,
	prefundedPrivateKey string,
) error {
	// get teleporter relayer balance
	teleporterRelayerBalance, err := evm.GetAddressBalance(rpcURL, teleporterRelayerAddress)
	if err != nil {
		return err
	}
	if teleporterRelayerBalance.Cmp(teleporterRelayerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(teleporterRelayerRequiredBalance, teleporterRelayerBalance)
		err := evm.FundAddress(
			rpcURL,
			prefundedPrivateKey,
			teleporterRelayerAddress,
			toFund,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeployRelayer(
	app *application.Avalanche,
	version string,
	network models.Network,
	subnetsInfo []RelayerSubnetInfo,
	teleporterContractAddress string,
) error {
	binPath, err := installRelayer(app, version)
	if err != nil {
		return err
	}
	_ = binPath
	awmRelayerConfig, err := createRelayerConfig(
		logging.Info.LowerString(),
		app.GetAWMRelayerStorageDir(),
		network.ID,
		network.Endpoint,
		subnetsInfo,
		teleporterContractAddress,
		teleporterRelayerAddress,
		teleporterRelayerPrivateKey,
	)
	if err != nil {
		return err
	}
	bs, err := json.MarshalIndent(awmRelayerConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(app.GetAWMRelayerConfigPath(), bs, constants.WriteReadReadPerms); err != nil {
		return err
	}
	return nil
}

func installRelayer(app *application.Avalanche, version string) (string, error) {
	awmRelayerBinDir := app.GetAWMRelayerBinDir()
	binDir := filepath.Join(awmRelayerBinDir, version)
	binPath := filepath.Join(binDir, constants.AWMRelayerBin)
	if utils.IsExecutable(binPath) {
		ux.Logger.PrintToUser("AWM-Relayer %s is already installed", version)
		return binPath, nil
	}
	ux.Logger.PrintToUser("installing AWM-Relayer %s", version)
	url, err := getRelayerURL(version)
	if err != nil {
		return "", err
	}
	bs, err := utils.Download(url)
	if err != nil {
		return "", err
	}
	if err := binutils.InstallArchive("tar.gz", bs, binDir); err != nil {
		return "", err
	}
	return binPath, nil
}

func getRelayerURL(version string) (string, error) {
	goarch, goos := runtime.GOARCH, runtime.GOOS
	if goos != "linux" && goos != "darwin" {
		return "", fmt.Errorf("OS not supported: %s", goos)
	}
	trimmedVersion := strings.TrimPrefix(version, "v")
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/awm-relayer_%s_%s_%s.tar.gz",
		constants.AvaLabsOrg,
		constants.AWMRelayerRepoName,
		version,
		trimmedVersion,
		goos,
		goarch,
	), nil
}

type RelayerSubnetInfo struct {
	SubnetID                  string
	BlockchainID              string
	TeleporterRegistryAddress string
}

// Get the host and port from a URI. The URI should be in the format http://host:port or https://host:port or host:port
func getURIHostAndPort(uri string) (string, uint32, error) {
	trimmedUri := uri
	trimmedUri = strings.TrimPrefix(trimmedUri, "http://")
	trimmedUri = strings.TrimPrefix(trimmedUri, "https://")
	hostAndPort := strings.Split(trimmedUri, ":")
	if len(hostAndPort) != 2 {
		return "", 0, fmt.Errorf("expected only host and port fields in %s", uri)
	}
	port, err := strconv.ParseUint(hostAndPort[1], 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port from %s: %w", uri, err)
	}
	return hostAndPort[0], uint32(port), nil
}

// Constructs a relayer config with all subnets as sources and destinations
func createRelayerConfig(
	logLevel string,
	storageLocation string,
	networkID uint32,
	endpoint string,
	subnetsInfo []RelayerSubnetInfo,
	teleporterContractAddress string,
	relayerRewardAddress string,
	relayerFundedAddressKey string,
) (config.Config, error) {
	host, port, err := getURIHostAndPort(endpoint)
	if err != nil {
		return config.Config{}, err
	}
	sources := make([]*config.SourceSubnet, len(subnetsInfo))
	destinations := make([]*config.DestinationSubnet, len(subnetsInfo))
	for i, subnetInfo := range subnetsInfo {
		sources[i] = &config.SourceSubnet{
			SubnetID:          subnetInfo.SubnetID,
			BlockchainID:      subnetInfo.BlockchainID,
			VM:                config.EVM.String(),
			EncryptConnection: false,
			APINodeHost:       host,
			APINodePort:       port,
			MessageContracts: map[string]config.MessageProtocolConfig{
				teleporterContractAddress: {
					MessageFormat: config.TELEPORTER.String(),
					Settings: map[string]interface{}{
						"reward-address": relayerRewardAddress,
					},
				},
				offchainregistry.OffChainRegistrySourceAddress.Hex(): {
					MessageFormat: config.OFF_CHAIN_REGISTRY.String(),
					Settings: map[string]interface{}{
						"teleporter-registry-address": subnetInfo.TeleporterRegistryAddress,
					},
				},
			},
		}
		destinations[i] = &config.DestinationSubnet{
			SubnetID:          subnetInfo.SubnetID,
			BlockchainID:      subnetInfo.BlockchainID,
			VM:                config.EVM.String(),
			EncryptConnection: false,
			APINodeHost:       host,
			APINodePort:       port,
			AccountPrivateKey: relayerFundedAddressKey,
		}
	}
	return config.Config{
		LogLevel:            logLevel,
		NetworkID:           networkID,
		PChainAPIURL:        endpoint,
		EncryptConnection:   false,
		StorageLocation:     storageLocation,
		ProcessMissedBlocks: false,
		SourceSubnets:       sources,
		DestinationSubnets:  destinations,
	}, nil
}

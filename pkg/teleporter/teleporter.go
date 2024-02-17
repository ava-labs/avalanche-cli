// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"fmt"
	"math/big"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/awm-relayer/config"
	teleporterRegistry "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/upgrades/TeleporterRegistry"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// TODO: use abi without any download?
	teleporterReleaseURL                     = "https://github.com/ava-labs/teleporter/releases/download/%s/"
	teleporterMessengerContractAddressURLFmt = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_%s.txt"
	teleporterMessengerDeployerAddressURLFmt = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_%s.txt"
	teleporterMessengerDeployerTxURLFmt      = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_%s.txt"
	teleporterRelayerPrivateKey              = "C2CE4E001B7585F543982A01FBC537CFF261A672FA8BD1FAFC08A207098FE2DE"
	teleporterRelayerAddress                 = "0xA100fF48a37cab9f87c8b5Da933DA46ea1a5fb80"
)

var (
	teleporterMessengerDeployerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))  // 10 AVAX
	teleporterRelayerRequiredBalance           = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX
	TeleporterPrefundedAddressBalance          = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(600)) // 600 AVAX
)

func getTeleporterURLs(version string) (string, string, string) {
	teleporterMessengerContractAddressURL := fmt.Sprintf(teleporterMessengerContractAddressURLFmt, version, version)
	teleporterMessengerDeployerAddressURL := fmt.Sprintf(teleporterMessengerDeployerAddressURLFmt, version, version)
	teleporterMessengerDeployerTxURL := fmt.Sprintf(teleporterMessengerDeployerTxURLFmt, version, version)
	return teleporterMessengerContractAddressURL, teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerTxURL
}

type Deployer struct {
	teleporterMessengerContractAddress string
	teleporterMessengerDeployerAddress string
	teleporterMessengerDeployerTx      string
}

func (t *Deployer) downloadAssets(version string) error {
	var err error
	teleporterMessengerContractAddressURL, teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerTxURL := getTeleporterURLs(version)
	if t.teleporterMessengerContractAddress == "" {
		// get target teleporter messenger contract address
		t.teleporterMessengerContractAddress, err = utils.DownloadStr(teleporterMessengerContractAddressURL)
		if err != nil {
			return err
		}
	}
	if t.teleporterMessengerDeployerAddress == "" {
		// get teleporter deployer address
		t.teleporterMessengerDeployerAddress, err = utils.DownloadStr(teleporterMessengerDeployerAddressURL)
		if err != nil {
			return err
		}
	}
	if t.teleporterMessengerDeployerTx == "" {
		t.teleporterMessengerDeployerTx, err = utils.DownloadStr(teleporterMessengerDeployerTxURL)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Deployer) Deploy(
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (string, string, error) {
	messengerAddress, err := t.DeployMessenger(version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	registryAddress, err := t.DeployRegistry(version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	if err := t.FundRelayer(rpcURL, prefundedPrivateKey); err != nil {
		return "", "", err
	}
	return messengerAddress, registryAddress, nil
}

func (t *Deployer) DeployMessenger(version string, subnetName string, rpcURL string, prefundedPrivateKey string) (string, error) {
	t.downloadAssets(version)
	// check if contract is already deployed
	teleporterMessengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(rpcURL, t.teleporterMessengerContractAddress)
	if err != nil {
		return "", err
	}
	if teleporterMessengerAlreadyDeployed {
		ux.Logger.PrintToUser("Teleporter Messenger has already been deployed to %s", subnetName)
		return t.teleporterMessengerContractAddress, nil
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := evm.GetAddressBalance(rpcURL, t.teleporterMessengerDeployerAddress)
	if err != nil {
		return "", err
	}
	if teleporterMessengerDeployerBalance.Cmp(teleporterMessengerDeployerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(teleporterMessengerDeployerRequiredBalance, teleporterMessengerDeployerBalance)
		err := evm.FundAddress(
			rpcURL,
			prefundedPrivateKey,
			t.teleporterMessengerDeployerAddress,
			toFund,
		)
		if err != nil {
			return "", err
		}
	}
	if err := evm.IssueTx(rpcURL, t.teleporterMessengerDeployerTx); err != nil {
		return "", err
	}
	ux.Logger.PrintToUser("Teleporter Messenger successfully deployed to %s", subnetName)
	return t.teleporterMessengerContractAddress, nil
}

func (t *Deployer) DeployRegistry(version string, subnetName string, rpcURL string, prefundedPrivateKey string) (string, error) {
	t.downloadAssets(version)
	teleporterMessengerContractAddress := common.HexToAddress(t.teleporterMessengerContractAddress)
	teleporterRegistryConstructorInput := []teleporterRegistry.ProtocolRegistryEntry{
		{
			Version:         big.NewInt(1),
			ProtocolAddress: teleporterMessengerContractAddress,
		},
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return "", err
	}
	defer client.Close()
	signer, err := evm.GetSigner(client, prefundedPrivateKey)
	if err != nil {
		return "", err
	}
	teleporterRegistryAddress, tx, _, err := teleporterRegistry.DeployTeleporterRegistry(signer, client, teleporterRegistryConstructorInput)
	if err != nil {
		return "", err
	}
	_, success, err := evm.WaitForTransaction(client, tx)
	if err != nil {
		return "", err
	}
	if !success {
		return "", fmt.Errorf("failed receipt status deploying teleporter registry")
	}
	ux.Logger.PrintToUser("Teleporter Registry successfully deployed to %s", subnetName)
	return teleporterRegistryAddress.String(), nil
}

func (t *Deployer) FundRelayer(rpcURL string, prefundedPrivateKey string) error {
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

func DeployAWMRelayer(app *application.Avalanche, version string) error {
	binPath, err := installAWMRelayer(app, version)
	configPath := app.GetAWMRelayerConfigPath()
	fmt.Println(binPath)
	fmt.Println(configPath)
	return err
}

func installAWMRelayer(app *application.Avalanche, version string) (string, error) {
	awmRelayerBinDir := app.GetAWMRelayerBinDir()
	binDir := filepath.Join(awmRelayerBinDir, version)
	binPath := filepath.Join(binDir, constants.AWMRelayerBin)
	if utils.IsExecutable(binPath) {
		ux.Logger.PrintToUser("AWM-Relayer %s is already installed", version)
		return binPath, nil
	}
	ux.Logger.PrintToUser("installing AWM-Relayer %s", version)
	url, err := getAWMRelayerURL(version)
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

func getAWMRelayerURL(version string) (string, error) {
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

type SubnetInfo struct {
	/*
	   SubnetID            ids.ID
	   BlockchainID        ids.ID
	   NodeURIs            []string
	   WSClient            ethclient.Client
	   RPCClient           ethclient.Client
	   EVMChainID          *big.Int
	   TeleporterRegistry  *teleporterregistry.TeleporterRegistry
	   TeleporterMessenger *teleportermessenger.TeleporterMessenger
	   // TeleporterRegistryAddress is unique across subnets.
	   TeleporterRegistryAddress common.Address
	*/
}

// Constructs a relayer config with all subnets as sources and destinations
func createAWMRelayerConfig(
	logLevel string,
	storageLocation string,
	networkID uint32,
	pChainEndpoint string,
	subnetsInfo []SubnetInfo,
	//teleporterContractAddress common.Address,
	//fundedAddress common.Address,
	//relayerKey *ecdsa.PrivateKey,
) config.Config {
	sources := make([]*config.SourceSubnet, len(subnetsInfo))
	destinations := make([]*config.DestinationSubnet, len(subnetsInfo))
	/*
		for i, subnetInfo := range subnetsInfo {
			host, port, err := teleporterTestUtils.GetURIHostAndPort(subnetInfo.NodeURIs[0])
			Expect(err).Should(BeNil())

			sources[i] = &config.SourceSubnet{
				SubnetID:          subnetInfo.SubnetID.String(),
				BlockchainID:      subnetInfo.BlockchainID.String(),
				VM:                config.EVM.String(),
				EncryptConnection: false,
				APINodeHost:       host,
				APINodePort:       port,
				MessageContracts: map[string]config.MessageProtocolConfig{
					teleporterContractAddress.Hex(): {
						MessageFormat: config.TELEPORTER.String(),
						Settings: map[string]interface{}{
							"reward-address": fundedAddress.Hex(),
						},
					},
					offchainregistry.OffChainRegistrySourceAddress.Hex(): {
						MessageFormat: config.OFF_CHAIN_REGISTRY.String(),
						Settings: map[string]interface{}{
							"teleporter-registry-address": subnetInfo.TeleporterRegistryAddress.Hex(),
						},
					},
				},
			}

			destinations[i] = &config.DestinationSubnet{
				SubnetID:          subnetInfo.SubnetID.String(),
				BlockchainID:      subnetInfo.BlockchainID.String(),
				VM:                config.EVM.String(),
				EncryptConnection: false,
				APINodeHost:       host,
				APINodePort:       port,
				AccountPrivateKey: hex.EncodeToString(relayerKey.D.Bytes()),
			}

			log.Info(
				"Creating relayer config for subnet",
				"subnetID", subnetInfo.SubnetID.String(),
				"blockchainID", subnetInfo.BlockchainID.String(),
				"host", host,
				"port", port,
			)
		}
	*/

	return config.Config{
		LogLevel:            logLevel,
		NetworkID:           networkID,
		PChainAPIURL:        pChainEndpoint,
		EncryptConnection:   false,
		StorageLocation:     storageLocation,
		ProcessMissedBlocks: false,
		SourceSubnets:       sources,
		DestinationSubnets:  destinations,
	}
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	teleporterRegistry "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/upgrades/TeleporterRegistry"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// TODO: use abi without any download?
	teleporterReleaseURL                     = "https://github.com/ava-labs/teleporter/releases/download/%s/"
	teleporterMessengerContractAddressURLFmt = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_%s.txt"
	teleporterMessengerDeployerAddressURLFmt = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_%s.txt"
	teleporterMessengerDeployerTxURLFmt      = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_%s.txt"
)

var (
	teleporterMessengerDeployerRequiredBalance = big.NewInt(0).
							Mul(big.NewInt(1e18), big.NewInt(10))
	// 10 AVAX
	TeleporterPrefundedAddressBalance = big.NewInt(0).
						Mul(big.NewInt(1e18), big.NewInt(600))
	// 600 AVAX
)

func getTeleporterURLs(version string) (string, string, string) {
	teleporterMessengerContractAddressURL := fmt.Sprintf(
		teleporterMessengerContractAddressURLFmt,
		version,
		version,
	)
	teleporterMessengerDeployerAddressURL := fmt.Sprintf(
		teleporterMessengerDeployerAddressURLFmt,
		version,
		version,
	)
	teleporterMessengerDeployerTxURL := fmt.Sprintf(
		teleporterMessengerDeployerTxURLFmt,
		version,
		version,
	)
	return teleporterMessengerContractAddressURL, teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerTxURL
}

type Deployer struct {
	teleporterMessengerContractAddress string
	teleporterMessengerDeployerAddress string
	teleporterMessengerDeployerTx      string
}

func (t *Deployer) GetAssets(
	teleporterInstallDir string,
	version string,
) (string, string, string, error) {
	if err := t.DownloadAssets(teleporterInstallDir, version); err != nil {
		return "", "", "", err
	}
	return t.teleporterMessengerContractAddress, t.teleporterMessengerDeployerAddress, t.teleporterMessengerDeployerTx, nil
}

func (t *Deployer) DownloadAssets(
	teleporterInstallDir string,
	version string,
) error {
	var err error
	binDir := filepath.Join(teleporterInstallDir, version)
	teleporterMessengerContractAddressURL, teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerTxURL := getTeleporterURLs(
		version,
	)
	teleporterMessengerContractAddressPath := filepath.Join(
		binDir,
		filepath.Base(teleporterMessengerContractAddressURL),
	)
	teleporterMessengerDeployerAddressPath := filepath.Join(
		binDir,
		filepath.Base(teleporterMessengerDeployerAddressURL),
	)
	teleporterMessengerDeployerTxPath := filepath.Join(
		binDir,
		filepath.Base(teleporterMessengerDeployerTxURL),
	)
	if t.teleporterMessengerContractAddress == "" {
		var teleporterMessengerContractAddressBytes []byte
		if utils.FileExists(teleporterMessengerContractAddressPath) {
			teleporterMessengerContractAddressBytes, err = os.ReadFile(
				teleporterMessengerContractAddressPath,
			)
			if err != nil {
				return err
			}
		} else {
			// get target teleporter messenger contract address
			teleporterMessengerContractAddressBytes, err = utils.DownloadWithTee(teleporterMessengerContractAddressURL, teleporterMessengerContractAddressPath)
			if err != nil {
				return err
			}
		}
		t.teleporterMessengerContractAddress = string(teleporterMessengerContractAddressBytes)
	}
	if t.teleporterMessengerDeployerAddress == "" {
		var teleporterMessengerDeployerAddressBytes []byte
		if utils.FileExists(teleporterMessengerDeployerAddressPath) {
			teleporterMessengerDeployerAddressBytes, err = os.ReadFile(
				teleporterMessengerDeployerAddressPath,
			)
			if err != nil {
				return err
			}
		} else {
			// get teleporter deployer address
			teleporterMessengerDeployerAddressBytes, err = utils.DownloadWithTee(teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerAddressPath)
			if err != nil {
				return err
			}
		}
		t.teleporterMessengerDeployerAddress = string(teleporterMessengerDeployerAddressBytes)
	}
	if t.teleporterMessengerDeployerTx == "" {
		var teleporterMessengerDeployerTxBytes []byte
		if utils.FileExists(teleporterMessengerDeployerTxPath) {
			teleporterMessengerDeployerTxBytes, err = os.ReadFile(teleporterMessengerDeployerTxPath)
			if err != nil {
				return err
			}
		} else {
			teleporterMessengerDeployerTxBytes, err = utils.DownloadWithTee(teleporterMessengerDeployerTxURL, teleporterMessengerDeployerTxPath)
			if err != nil {
				return err
			}
		}
		t.teleporterMessengerDeployerTx = string(teleporterMessengerDeployerTxBytes)
	}
	return nil
}

func (t *Deployer) Deploy(
	teleporterInstallDir string,
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
	deployMessenger bool,
	deployRegistry bool,
) (bool, string, string, error) {
	var (
		messengerAddress string
		registryAddress  string
		alreadyDeployed  bool
		err              error
	)
	if deployMessenger {
		alreadyDeployed, messengerAddress, err = t.DeployMessenger(
			teleporterInstallDir,
			version,
			subnetName,
			rpcURL,
			prefundedPrivateKey,
		)
	}
	if err == nil && deployRegistry {
		if !deployMessenger || !alreadyDeployed {
			registryAddress, err = t.DeployRegistry(teleporterInstallDir, version, subnetName, rpcURL, prefundedPrivateKey)
		}
	}
	return alreadyDeployed, messengerAddress, registryAddress, err
}

func (t *Deployer) DeployMessenger(
	teleporterInstallDir string,
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (bool, string, error) {
	if err := t.DownloadAssets(teleporterInstallDir, version); err != nil {
		return false, "", err
	}
	// check if contract is already deployed
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return false, "", err
	}
	if teleporterMessengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(client, t.teleporterMessengerContractAddress); err != nil {
		return false, "", fmt.Errorf("failure making a request to %s: %w", rpcURL, err)
	} else if teleporterMessengerAlreadyDeployed {
		ux.Logger.PrintToUser("Teleporter Messenger has already been deployed to %s", subnetName)
		return true, t.teleporterMessengerContractAddress, nil
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := evm.GetAddressBalance(
		client,
		t.teleporterMessengerDeployerAddress,
	)
	if err != nil {
		return false, "", err
	}
	if teleporterMessengerDeployerBalance.Cmp(teleporterMessengerDeployerRequiredBalance) < 0 {
		toFund := big.NewInt(0).
			Sub(teleporterMessengerDeployerRequiredBalance, teleporterMessengerDeployerBalance)
		if err := evm.FundAddress(
			client,
			prefundedPrivateKey,
			t.teleporterMessengerDeployerAddress,
			toFund,
		); err != nil {
			return false, "", err
		}
	}
	if err := evm.IssueTx(client, t.teleporterMessengerDeployerTx); err != nil {
		return false, "", err
	}
	ux.Logger.PrintToUser(
		"Teleporter Messenger successfully deployed to %s (%s)",
		subnetName,
		t.teleporterMessengerContractAddress,
	)
	return false, t.teleporterMessengerContractAddress, nil
}

func (t *Deployer) DeployRegistry(
	teleporterInstallDir string,
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (string, error) {
	if err := t.DownloadAssets(teleporterInstallDir, version); err != nil {
		return "", err
	}
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
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return "", err
	}
	teleporterRegistryAddress, tx, _, err := DeployTeleporterRegistry(
		txOpts,
		client,
		teleporterRegistryConstructorInput,
	)
	if err != nil {
		return "", err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return "", err
	} else if !success {
		return "", fmt.Errorf("failed receipt status deploying teleporter registry")
	}
	ux.Logger.PrintToUser(
		"Teleporter Registry successfully deployed to %s (%s)",
		subnetName,
		teleporterRegistryAddress,
	)
	return teleporterRegistryAddress.String(), nil
}

func DeployTeleporterRegistry(
	txOpts *bind.TransactOpts,
	client ethclient.Client,
	teleporterRegistryConstructorInput []teleporterRegistry.ProtocolRegistryEntry,
) (common.Address, *types.Transaction, *teleporterRegistry.TeleporterRegistry, error) {
	const (
		repeatsOnFailure    = 3
		sleepBetweenRepeats = 1 * time.Second
	)
	var (
		addr     common.Address
		tx       *types.Transaction
		registry *teleporterRegistry.TeleporterRegistry
		err      error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		txOpts.Context = ctx
		addr, tx, registry, err = teleporterRegistry.DeployTeleporterRegistry(
			txOpts,
			client,
			teleporterRegistryConstructorInput,
		)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure deploying teleporter registry on %#v: %w", client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return addr, tx, registry, err
}

func getPrivateKey(
	app *application.Avalanche,
	network models.Network,
	keyName string,
) (string, error) {
	var (
		err error
		k   *key.SoftKey
	)
	if keyName == "" {
		if k, err = key.LoadEwoq(network.ID); err != nil {
			return "", err
		}
	} else {
		k, err = key.LoadSoft(network.ID, app.GetKeyPath(keyName))
		if err != nil {
			return "", err
		}
	}
	return k.PrivKeyHex(), nil
}

func SetProposerVM(
	app *application.Avalanche,
	network models.Network,
	blockchainID string,
	fundedKeyName string,
) error {
	privKeyStr, err := getPrivateKey(app, network, fundedKeyName)
	if err != nil {
		return err
	}
	wsEndpoint := network.BlockchainWSEndpoint(blockchainID)
	return evm.SetupProposerVM(wsEndpoint, privKeyStr)
}

func DeployAndFundRelayer(
	app *application.Avalanche,
	teleporterVersion string,
	network models.Network,
	subnetName string,
	blockchainID string,
	fundedKeyName string,
) (bool, string, string, error) {
	privKeyStr, err := getPrivateKey(app, network, fundedKeyName)
	if err != nil {
		return false, "", "", err
	}
	endpoint := network.BlockchainEndpoint(blockchainID)
	td := Deployer{}
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
		app.GetTeleporterBinDir(),
		teleporterVersion,
		subnetName,
		endpoint,
		privKeyStr,
		true,
		true,
	)
	if err != nil {
		return false, "", "", err
	}
	if !alreadyDeployed {
		// get relayer address
		relayerAddress, _, err := GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
		if err != nil {
			return false, "", "", err
		}
		// fund relayer
		if err := FundRelayer(
			endpoint,
			privKeyStr,
			relayerAddress,
		); err != nil {
			return false, "", "", err
		}
	}
	return alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err
}

func getTeleporterKeyInfo(
	app *application.Avalanche,
	keyName string,
) (string, string, *big.Int, error) {
	k, err := key.LoadSoftOrCreate(models.NewLocalNetwork().ID, app.GetKeyPath(keyName))
	if err != nil {
		return "", "", nil, err
	}
	return k.C(), k.PrivKeyHex(), TeleporterPrefundedAddressBalance, nil
}

type Info struct {
	Version                  string
	FundedAddress            string
	FundedBalance            *big.Int
	MessengerDeployerAddress string
	RelayerAddress           string
}

func GetInfo(
	app *application.Avalanche,
) (*Info, error) {
	var err error
	ti := Info{}
	ti.FundedAddress, _, ti.FundedBalance, err = getTeleporterKeyInfo(
		app,
		constants.TeleporterKeyName,
	)
	if err != nil {
		return nil, err
	}
	ti.Version, err = app.Downloader.GetLatestReleaseVersion(
		binutils.GetGithubLatestReleaseURL(constants.AvaLabsOrg, constants.TeleporterRepoName),
	)
	if err != nil {
		return nil, err
	}
	deployer := Deployer{}
	_, ti.MessengerDeployerAddress, _, err = deployer.GetAssets(
		app.GetTeleporterBinDir(),
		ti.Version,
	)
	if err != nil {
		return nil, err
	}
	ti.RelayerAddress, _, err = GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
	if err != nil {
		return nil, err
	}
	return &ti, nil
}

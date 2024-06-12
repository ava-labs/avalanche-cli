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
	registry "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/upgrades/TeleporterRegistry"
	"github.com/ethereum/go-ethereum/common"
)

const (
	releaseURL                     = "https://github.com/ava-labs/teleporter/releases/download/%s/"
	messengerContractAddressURLFmt = releaseURL + "/TeleporterMessenger_Contract_Address_%s.txt"
	messengerDeployerAddressURLFmt = releaseURL + "/TeleporterMessenger_Deployer_Address_%s.txt"
	messengerDeployerTxURLFmt      = releaseURL + "/TeleporterMessenger_Deployment_Transaction_%s.txt"
	registryBytecodeURLFmt         = releaseURL + "/TeleporterRegisrtry_Bytecode_%s.txt"
)

var (
	messengerDeployerRequiredBalance = big.NewInt(0).
						Mul(big.NewInt(1e18), big.NewInt(10))
	// 10 AVAX
	TeleporterPrefundedAddressBalance = big.NewInt(0).
						Mul(big.NewInt(1e18), big.NewInt(600))
	// 600 AVAX
)

func getTeleporterURLs(version string) (string, string, string, string) {
	messengerContractAddressURL := fmt.Sprintf(
		messengerContractAddressURLFmt,
		version,
		version,
	)
	messengerDeployerAddressURL := fmt.Sprintf(
		messengerDeployerAddressURLFmt,
		version,
		version,
	)
	messengerDeployerTxURL := fmt.Sprintf(
		messengerDeployerTxURLFmt,
		version,
		version,
	)
	registryBydecodeURL := fmt.Sprintf(
		registryBytecodeURLFmt,
		version,
		version,
	)
	return messengerContractAddressURL, messengerDeployerAddressURL, messengerDeployerTxURL, registryBydecodeURL
}

type Deployer struct {
	messengerContractAddress string
	messengerDeployerAddress string
	messengerDeployerTx      string
	registryBydecode         string
}

func (t *Deployer) GetAssets(
	teleporterInstallDir string,
	version string,
) (string, string, string, string, error) {
	if err := t.DownloadAssets(teleporterInstallDir, version); err != nil {
		return "", "", "", "", err
	}
	return t.messengerContractAddress, t.messengerDeployerAddress, t.messengerDeployerTx, t.registryBydecode, nil
}

func (t *Deployer) DownloadAssets(
	teleporterInstallDir string,
	version string,
) error {
	var err error
	binDir := filepath.Join(teleporterInstallDir, version)
	messengerContractAddressURL, messengerDeployerAddressURL, messengerDeployerTxURL, registryBydecodeURL := getTeleporterURLs(
		version,
	)
	messengerContractAddressPath := filepath.Join(
		binDir,
		filepath.Base(messengerContractAddressURL),
	)
	messengerDeployerAddressPath := filepath.Join(
		binDir,
		filepath.Base(messengerDeployerAddressURL),
	)
	messengerDeployerTxPath := filepath.Join(
		binDir,
		filepath.Base(messengerDeployerTxURL),
	)
	registryBytecodePath := filepath.Join(
		binDir,
		filepath.Base(registryBydecodeURL),
	)
	if t.messengerContractAddress == "" {
		var messengerContractAddressBytes []byte
		if utils.FileExists(messengerContractAddressPath) {
			messengerContractAddressBytes, err = os.ReadFile(
				messengerContractAddressPath,
			)
			if err != nil {
				return err
			}
		} else {
			// get target teleporter messenger contract address
			messengerContractAddressBytes, err = utils.DownloadWithTee(messengerContractAddressURL, messengerContractAddressPath)
			if err != nil {
				return err
			}
		}
		t.messengerContractAddress = string(messengerContractAddressBytes)
	}
	if t.messengerDeployerAddress == "" {
		var messengerDeployerAddressBytes []byte
		if utils.FileExists(messengerDeployerAddressPath) {
			messengerDeployerAddressBytes, err = os.ReadFile(
				messengerDeployerAddressPath,
			)
			if err != nil {
				return err
			}
		} else {
			// get teleporter deployer address
			messengerDeployerAddressBytes, err = utils.DownloadWithTee(messengerDeployerAddressURL, messengerDeployerAddressPath)
			if err != nil {
				return err
			}
		}
		t.messengerDeployerAddress = string(messengerDeployerAddressBytes)
	}
	if t.messengerDeployerTx == "" {
		var messengerDeployerTxBytes []byte
		if utils.FileExists(messengerDeployerTxPath) {
			messengerDeployerTxBytes, err = os.ReadFile(messengerDeployerTxPath)
			if err != nil {
				return err
			}
		} else {
			messengerDeployerTxBytes, err = utils.DownloadWithTee(messengerDeployerTxURL, messengerDeployerTxPath)
			if err != nil {
				return err
			}
		}
		t.messengerDeployerTx = string(messengerDeployerTxBytes)
	}
	if t.registryBydecode == "" {
		var registryBytecodeBytes []byte
		if utils.FileExists(registryBytecodePath) {
			registryBytecodeBytes, err = os.ReadFile(registryBytecodePath)
			if err != nil {
				return err
			}
		} else {
			registryBytecodeBytes, err = utils.DownloadWithTee(registryBydecodeURL, registryBytecodePath)
			if err != nil {
				return err
			}
		}
		t.registryBydecode = string(registryBytecodeBytes)
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
	if messengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(client, t.messengerContractAddress); err != nil {
		return false, "", fmt.Errorf("failure making a request to %s: %w", rpcURL, err)
	} else if messengerAlreadyDeployed {
		ux.Logger.PrintToUser("Teleporter Messenger has already been deployed to %s", subnetName)
		return true, t.messengerContractAddress, nil
	}
	// get teleporter deployer balance
	messengerDeployerBalance, err := evm.GetAddressBalance(
		client,
		t.messengerDeployerAddress,
	)
	if err != nil {
		return false, "", err
	}
	if messengerDeployerBalance.Cmp(messengerDeployerRequiredBalance) < 0 {
		toFund := big.NewInt(0).
			Sub(messengerDeployerRequiredBalance, messengerDeployerBalance)
		if err := evm.FundAddress(
			client,
			prefundedPrivateKey,
			t.messengerDeployerAddress,
			toFund,
		); err != nil {
			return false, "", err
		}
	}
	if err := evm.IssueTx(client, t.messengerDeployerTx); err != nil {
		return false, "", err
	}
	ux.Logger.PrintToUser(
		"Teleporter Messenger successfully deployed to %s (%s)",
		subnetName,
		t.messengerContractAddress,
	)
	return false, t.messengerContractAddress, nil
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
	messengerContractAddress := common.HexToAddress(t.messengerContractAddress)
	registryConstructorInput := []registry.ProtocolRegistryEntry{
		{
			Version:         big.NewInt(1),
			ProtocolAddress: messengerContractAddress,
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
	registryAddress, tx, _, err := DeployTeleporterRegistry(
		txOpts,
		client,
		registryConstructorInput,
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
		registryAddress,
	)
	return registryAddress.String(), nil
}

func DeployTeleporterRegistry(
	txOpts *bind.TransactOpts,
	client ethclient.Client,
	registryConstructorInput []registry.ProtocolRegistryEntry,
) (common.Address, *types.Transaction, *registry.TeleporterRegistry, error) {
	const (
		repeatsOnFailure    = 3
		sleepBetweenRepeats = 1 * time.Second
	)
	var (
		addr     common.Address
		tx       *types.Transaction
		registry *registry.TeleporterRegistry
		err      error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		txOpts.Context = ctx
		addr, tx, registry, err = registry.DeployTeleporterRegistry(
			txOpts,
			client,
			registryConstructorInput,
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
	alreadyDeployed, messengerAddress, registryAddress, err := td.Deploy(
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
	return alreadyDeployed, messengerAddress, registryAddress, err
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
	_, ti.MessengerDeployerAddress, _, _, err = deployer.GetAssets(
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

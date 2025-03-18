// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package interchain

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ethereum/go-ethereum/common"
)

const (
	releaseURL                     = "https://github.com/ava-labs/icm-contracts/releases/download/%s/"
	messengerContractAddressURLFmt = releaseURL + "/TeleporterMessenger_Contract_Address_%s.txt"
	messengerDeployerAddressURLFmt = releaseURL + "/TeleporterMessenger_Deployer_Address_%s.txt"
	messengerDeployerTxURLFmt      = releaseURL + "/TeleporterMessenger_Deployment_Transaction_%s.txt"
	registryBytecodeURLFmt         = releaseURL + "/TeleporterRegistry_Bytecode_%s.txt"
)

var (
	// 10 AVAX
	messengerDeployerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))
	// 600 AVAX
	InterchainMessagingPrefundedAddressBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(600))
)

func getICMURLs(version string) (string, string, string, string) {
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

type ICMDeployer struct {
	messengerContractAddress string
	messengerDeployerAddress string
	messengerDeployerTx      string
	registryBydecode         string
}

func (t *ICMDeployer) GetAssets(
	icmInstallDir string,
	version string,
) (string, string, string, string, error) {
	if err := t.DownloadAssets(icmInstallDir, version); err != nil {
		return "", "", "", "", err
	}
	return t.messengerContractAddress, t.messengerDeployerAddress, t.messengerDeployerTx, t.registryBydecode, nil
}

func (t *ICMDeployer) CheckAssets() error {
	if t.messengerContractAddress == "" || t.messengerDeployerAddress == "" || t.messengerDeployerTx == "" || t.registryBydecode == "" {
		return fmt.Errorf("icm assets has not been initialized")
	}
	return nil
}

func (t *ICMDeployer) SetAssetsFromPaths(
	messengerContractAddressPath string,
	messengerDeployerAddressPath string,
	messengerDeployerTxPath string,
	registryBydecodePath string,
) error {
	if messengerContractAddressPath != "" {
		if bs, err := os.ReadFile(messengerContractAddressPath); err != nil {
			return err
		} else {
			t.messengerContractAddress = string(bs)
		}
	}
	if messengerDeployerAddressPath != "" {
		if bs, err := os.ReadFile(messengerDeployerAddressPath); err != nil {
			return err
		} else {
			t.messengerDeployerAddress = string(bs)
		}
	}
	if messengerDeployerTxPath != "" {
		if bs, err := os.ReadFile(messengerDeployerTxPath); err != nil {
			return err
		} else {
			t.messengerDeployerTx = string(bs)
		}
	}
	if registryBydecodePath != "" {
		if bs, err := os.ReadFile(registryBydecodePath); err != nil {
			return err
		} else {
			t.registryBydecode = string(bs)
		}
	}
	return nil
}

func (t *ICMDeployer) SetAssets(
	messengerContractAddress string,
	messengerDeployerAddress string,
	messengerDeployerTx string,
	registryBydecode string,
) {
	if messengerContractAddress != "" {
		t.messengerContractAddress = messengerContractAddress
	}
	if messengerDeployerAddress != "" {
		t.messengerDeployerAddress = messengerDeployerAddress
	}
	if messengerDeployerTx != "" {
		t.messengerDeployerTx = messengerDeployerTx
	}
	if registryBydecode != "" {
		t.registryBydecode = registryBydecode
	}
}

func (t *ICMDeployer) DownloadAssets(
	icmInstallDir string,
	version string,
) error {
	var err error
	binDir := filepath.Join(icmInstallDir, version)
	messengerContractAddressURL, messengerDeployerAddressURL, messengerDeployerTxURL, registryBydecodeURL := getICMURLs(
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
			// get target icm messenger contract address
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
			// get icm deployer address
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

func (t *ICMDeployer) Deploy(
	subnetName string,
	rpcURL string,
	privateKey string,
	deployMessenger bool,
	deployRegistry bool,
	forceRegistryDeploy bool,
) (bool, string, string, error) {
	var (
		messengerAddress string
		registryAddress  string
		alreadyDeployed  bool
		err              error
	)
	if deployMessenger {
		alreadyDeployed, messengerAddress, err = t.DeployMessenger(
			subnetName,
			rpcURL,
			privateKey,
		)
	}
	if err == nil && deployRegistry {
		if !deployMessenger || !alreadyDeployed || forceRegistryDeploy {
			registryAddress, err = t.DeployRegistry(subnetName, rpcURL, privateKey)
		}
	}
	return alreadyDeployed, messengerAddress, registryAddress, err
}

func (t *ICMDeployer) DeployMessenger(
	subnetName string,
	rpcURL string,
	privateKey string,
) (bool, string, error) {
	if err := t.CheckAssets(); err != nil {
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
		ux.Logger.PrintToUser("ICM Messenger has already been deployed to %s", subnetName)
		return true, t.messengerContractAddress, nil
	}
	// get icm deployer balance
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
			privateKey,
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
		"ICM Messenger successfully deployed to %s (%s)",
		subnetName,
		t.messengerContractAddress,
	)
	return false, t.messengerContractAddress, nil
}

func (t *ICMDeployer) DeployRegistry(
	subnetName string,
	rpcURL string,
	privateKey string,
) (string, error) {
	if err := t.CheckAssets(); err != nil {
		return "", err
	}
	messengerContractAddress := common.HexToAddress(t.messengerContractAddress)
	type ProtocolRegistryEntry struct {
		Version         *big.Int
		ProtocolAddress common.Address
	}
	constructorInput := []ProtocolRegistryEntry{
		{
			Version:         big.NewInt(1),
			ProtocolAddress: messengerContractAddress,
		},
	}
	registryAddress, err := contract.DeployContract(
		rpcURL,
		privateKey,
		[]byte(t.registryBydecode),
		"([(uint256, address)])",
		constructorInput,
	)
	if err != nil {
		return "", err
	}
	ux.Logger.PrintToUser(
		"ICM Registry successfully deployed to %s (%s)",
		subnetName,
		registryAddress,
	)
	return registryAddress.Hex(), nil
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

func getICMKeyInfo(
	app *application.Avalanche,
	keyName string,
) (string, string, *big.Int, error) {
	k, err := key.LoadSoftOrCreate(models.NewLocalNetwork().ID, app.GetKeyPath(keyName))
	if err != nil {
		return "", "", nil, err
	}
	return k.C(), k.PrivKeyHex(), InterchainMessagingPrefundedAddressBalance, nil
}

type ICMInfo struct {
	Version                  string
	FundedAddress            string
	FundedBalance            *big.Int
	MessengerDeployerAddress string
	//RelayerAddress           string
}

func GetICMInfo(
	app *application.Avalanche,
) (*ICMInfo, error) {
	var err error
	ti := ICMInfo{}
	ti.FundedAddress, _, ti.FundedBalance, err = getICMKeyInfo(
		app,
		constants.ICMKeyName,
	)
	if err != nil {
		return nil, err
	}
	ti.Version = constants.ICMVersion
	deployer := ICMDeployer{}
	_, ti.MessengerDeployerAddress, _, _, err = deployer.GetAssets(
		app.GetICMContractsBinDir(),
		ti.Version,
	)
	if err != nil {
		return nil, err
	}
	/*
		ti.RelayerAddress, _, err = GetRelayerKeyInfo(app.GetKeyPath(constants.ICMRelayerKeyName))
		if err != nil {
			return nil, err
		}
	*/
	return &ti, nil
}

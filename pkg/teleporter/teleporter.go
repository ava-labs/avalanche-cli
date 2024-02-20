// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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
	teleporterMessengerDeployerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))  // 10 AVAX
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

func (t *Deployer) DownloadAssets(
	teleporterInstallDir string,
	version string,
) error {
	var err error
	binDir := filepath.Join(teleporterInstallDir, version)
	teleporterMessengerContractAddressURL, teleporterMessengerDeployerAddressURL, teleporterMessengerDeployerTxURL := getTeleporterURLs(version)
	teleporterMessengerContractAddressPath := filepath.Join(binDir, filepath.Base(teleporterMessengerContractAddressURL))
	teleporterMessengerDeployerAddressPath := filepath.Join(binDir, filepath.Base(teleporterMessengerDeployerAddressURL))
	teleporterMessengerDeployerTxPath := filepath.Join(binDir, filepath.Base(teleporterMessengerDeployerTxURL))
	if t.teleporterMessengerContractAddress == "" {
		var teleporterMessengerContractAddressBytes []byte
		if utils.FileExists(teleporterMessengerContractAddressPath) {
			teleporterMessengerContractAddressBytes, err = os.ReadFile(teleporterMessengerContractAddressPath)
		} else {
			// get target teleporter messenger contract address
			teleporterMessengerContractAddressBytes, err = utils.Download(teleporterMessengerContractAddressURL)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
				return err
			}
			if err := os.WriteFile(teleporterMessengerContractAddressPath, teleporterMessengerContractAddressBytes, constants.WriteReadReadPerms); err != nil {
				return err
			}
		}
		t.teleporterMessengerContractAddress = string(teleporterMessengerContractAddressBytes)
	}
	if t.teleporterMessengerDeployerAddress == "" {
		var teleporterMessengerDeployerAddressBytes []byte
		if utils.FileExists(teleporterMessengerDeployerAddressPath) {
			teleporterMessengerDeployerAddressBytes, err = os.ReadFile(teleporterMessengerDeployerAddressPath)
		} else {
			// get teleporter deployer address
			teleporterMessengerDeployerAddressBytes, err = utils.Download(teleporterMessengerDeployerAddressURL)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
				return err
			}
			if err := os.WriteFile(teleporterMessengerDeployerAddressPath, teleporterMessengerDeployerAddressBytes, constants.WriteReadReadPerms); err != nil {
				return err
			}
		}
		t.teleporterMessengerDeployerAddress = string(teleporterMessengerDeployerAddressBytes)
	}
	if t.teleporterMessengerDeployerTx == "" {
		var teleporterMessengerDeployerTxBytes []byte
		if utils.FileExists(teleporterMessengerDeployerTxPath) {
			teleporterMessengerDeployerTxBytes, err = os.ReadFile(teleporterMessengerDeployerTxPath)
		} else {
			teleporterMessengerDeployerTxBytes, err = utils.Download(teleporterMessengerDeployerTxURL)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
				return err
			}
			if err := os.WriteFile(teleporterMessengerDeployerTxPath, teleporterMessengerDeployerTxBytes, constants.WriteReadReadPerms); err != nil {
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
) (string, string, error) {
	alreadyDeployed, messengerAddress, err := t.DeployMessenger(teleporterInstallDir, version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	if alreadyDeployed {
		return messengerAddress, "", nil
	}
	registryAddress, err := t.DeployRegistry(teleporterInstallDir, version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	return messengerAddress, registryAddress, nil
}

func (t *Deployer) DeployMessenger(
	teleporterInstallDir string,
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (bool, string, error) {
	t.DownloadAssets(teleporterInstallDir, version)
	// check if contract is already deployed
	teleporterMessengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(rpcURL, t.teleporterMessengerContractAddress)
	if err != nil {
		return false, "", err
	}
	if teleporterMessengerAlreadyDeployed {
		ux.Logger.PrintToUser("Teleporter Messenger has already been deployed to %s", subnetName)
		return true, t.teleporterMessengerContractAddress, nil
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := evm.GetAddressBalance(rpcURL, t.teleporterMessengerDeployerAddress)
	if err != nil {
		return false, "", err
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
			return false, "", err
		}
	}
	if err := evm.IssueTx(rpcURL, t.teleporterMessengerDeployerTx); err != nil {
		return false, "", err
	}
	ux.Logger.PrintToUser("Teleporter Messenger successfully deployed to %s (%s)", subnetName, t.teleporterMessengerContractAddress)
	return false, t.teleporterMessengerContractAddress, nil
}

func (t *Deployer) DeployRegistry(
	teleporterInstallDir string,
	version string,
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (string, error) {
	t.DownloadAssets(teleporterInstallDir, version)
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
	ux.Logger.PrintToUser("Teleporter Registry successfully deployed to %s (%s)", subnetName, teleporterRegistryAddress)
	return teleporterRegistryAddress.String(), nil
}

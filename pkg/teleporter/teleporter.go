// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"fmt"
	"math/big"

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
	alreadyDeployed, messengerAddress, err := t.DeployMessenger(version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	if alreadyDeployed {
		return messengerAddress, "", nil
	}
	registryAddress, err := t.DeployRegistry(version, subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	if err := FundRelayer(rpcURL, prefundedPrivateKey); err != nil {
		return "", "", err
	}
	return messengerAddress, registryAddress, nil
}

func (t *Deployer) DeployMessenger(version string, subnetName string, rpcURL string, prefundedPrivateKey string) (bool, string, error) {
	t.downloadAssets(version)
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
	ux.Logger.PrintToUser("Teleporter Registry successfully deployed to %s (%s)", subnetName, teleporterRegistryAddress)
	return teleporterRegistryAddress.String(), nil
}

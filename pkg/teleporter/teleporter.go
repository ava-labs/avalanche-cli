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
	// TODO: use latest version
	// TODO: use abi without any download
	teleporterVersion                     = "v0.1.0"
	teleporterReleaseURL                  = "https://github.com/ava-labs/teleporter/releases/download/" + teleporterVersion + "/"
	teleporterMessengerContractAddressURL = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerAddressURL = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerTxURL      = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_" + teleporterVersion + ".txt"
	teleporterRelayerPrivateKey           = "C2CE4E001B7585F543982A01FBC537CFF261A672FA8BD1FAFC08A207098FE2DE"
	teleporterRelayerAddress              = "0xA100fF48a37cab9f87c8b5Da933DA46ea1a5fb80"
)

var (
	teleporterMessengerDeployerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))  // 10 AVAX
	teleporterRelayerRequiredBalance           = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX
)

func Deploy(
	subnetName string,
	rpcURL string,
	prefundedPrivateKey string,
) (string, string, error) {
	messengerAddress, err := DeployMessenger(subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	registryAddress, err := DeployRegistry(subnetName, rpcURL, prefundedPrivateKey)
	if err != nil {
		return "", "", err
	}
	if err := FundRelayer(rpcURL, prefundedPrivateKey); err != nil {
		return "", "", err
	}
	return messengerAddress, registryAddress, nil
}

func DeployMessenger(subnetName string, rpcURL string, prefundedPrivateKey string) (string, error) {
	// get target teleporter messenger contract address
	teleporterMessengerContractAddress, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return "", err
	}
	// check if contract is already deployed
	teleporterMessengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
	if err != nil {
		return "", err
	}
	if teleporterMessengerAlreadyDeployed {
		ux.Logger.PrintToUser("Teleporter Messenger has already been deployed to %s", subnetName)
		return teleporterMessengerContractAddress, nil
	}
	// get teleporter deployer address
	teleporterMessengerDeployerAddress, err := utils.DownloadStr(teleporterMessengerDeployerAddressURL)
	if err != nil {
		return "", err
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := evm.GetAddressBalance(rpcURL, teleporterMessengerDeployerAddress)
	if err != nil {
		return "", err
	}
	if teleporterMessengerDeployerBalance.Cmp(teleporterMessengerDeployerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(teleporterMessengerDeployerRequiredBalance, teleporterMessengerDeployerBalance)
		err := evm.FundAddress(
			rpcURL,
			prefundedPrivateKey,
			teleporterMessengerDeployerAddress,
			toFund,
		)
		if err != nil {
			return "", err
		}
	}
	teleporterMessengerDeployerTx, err := utils.DownloadStr(teleporterMessengerDeployerTxURL)
	if err != nil {
		return "", err
	}
	if err := evm.IssueTx(rpcURL, teleporterMessengerDeployerTx); err != nil {
		return "", err
	}
	ux.Logger.PrintToUser("Teleporter Messenger successfully deployed to %s", subnetName)
	return teleporterMessengerContractAddress, nil
}

func DeployRegistry(subnetName string, rpcURL string, prefundedPrivateKey string) (string, error) {
	// get constructor input
	teleporterMessengerContractAddressStr, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return "", err
	}
	teleporterMessengerContractAddress := common.HexToAddress(teleporterMessengerContractAddressStr)
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

func FundRelayer(rpcURL string, prefundedPrivateKey string) error {
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

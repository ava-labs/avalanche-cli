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
	teleporterVersion                          = "v0.1.0"
	teleporterReleaseURL                       = "https://github.com/ava-labs/teleporter/releases/download/" + teleporterVersion + "/"
	teleporterMessengerContractAddressURL      = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerAddressURL      = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerTxURL           = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerRequiredBalance = uint64(10000000000000000000) // 10 eth
)

func DeployRegistry(subnetName string, rpcURL string, prefundedPrivateKeyStr string) (common.Address, error) {
	// get constructor input
	teleporterMessengerContractAddressStr, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return common.Address{}, err
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
		return common.Address{}, err
	}
	signer, err := evm.GetSigner(client, prefundedPrivateKeyStr)
	if err != nil {
		return common.Address{}, err
	}
	teleporterRegistryAddress, tx, _, err := teleporterRegistry.DeployTeleporterRegistry(signer, client, teleporterRegistryConstructorInput)
	if err != nil {
		return common.Address{}, err
	}
	_, success, err := evm.WaitForTransaction(client, tx)
	if err != nil {
		return common.Address{}, err
	}
	if !success {
		return common.Address{}, fmt.Errorf("failed receipt status deploying teleporter registry")
	}
	return teleporterRegistryAddress, nil
}

func DeployMessenger(subnetName string, rpcURL string, prefundedPrivateKey string) error {
	if b, err := MessengerAlreadyDeployed(rpcURL); err != nil {
		return err
	} else if b {
		ux.Logger.PrintToUser("Teleporter has already been deployed to %s", subnetName)
		return nil
	}
	ux.Logger.PrintToUser("Deploying Teleporter into %s", subnetName)
	// get target teleporter messenger contract address
	teleporterMessengerContractAddress, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return err
	}
	// check if contract is already deployed
	teleporterMessengerAlreadyDeployed, err := evm.ContractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
	if err != nil {
		return err
	}
	if teleporterMessengerAlreadyDeployed {
		return nil
	}
	// get teleporter deployer address
	teleporterMessengerDeployerAddress, err := utils.DownloadStr(teleporterMessengerDeployerAddressURL)
	if err != nil {
		return err
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := evm.GetAddressBalance(rpcURL, teleporterMessengerDeployerAddress)
	if err != nil {
		return err
	}
	if teleporterMessengerDeployerBalance < teleporterMessengerDeployerRequiredBalance {
		toFund := teleporterMessengerDeployerRequiredBalance - teleporterMessengerDeployerBalance
		err := evm.FundAddress(
			rpcURL,
			prefundedPrivateKey,
			teleporterMessengerDeployerAddress,
			toFund,
		)
		if err != nil {
			return err
		}
	}
	teleporterMessengerDeployerTx, err := utils.DownloadStr(teleporterMessengerDeployerTxURL)
	if err != nil {
		return err
	}
	if err := evm.IssueTx(rpcURL, teleporterMessengerDeployerTx); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Teleporter successfully deployed to %s", subnetName)
	return nil
}

func MessengerAlreadyDeployed(rpcURL string) (bool, error) {
	teleporterMessengerContractAddress, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return false, err
	}
	return evm.ContractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
}

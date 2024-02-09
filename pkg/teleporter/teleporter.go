// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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

func DeployTeleporter(subnetName string, rpcURL string, prefundedPrivateKey string) error {
	if b, err := TeleporterAlreadyDeployed(rpcURL); err != nil {
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

func TeleporterAlreadyDeployed(rpcURL string) (bool, error) {
	teleporterMessengerContractAddress, err := utils.DownloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return false, err
	}
	return evm.ContractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
}

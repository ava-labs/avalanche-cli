// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package icm

import (
	"math/big"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanche-tooling-sdk-go/interchain/icm"
)

var InterchainMessagingPrefundedAddressBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(600)) // 600 AVAX

// getAssetPaths returns the local file paths for ICM deployment assets.
func getAssetPaths(binDir string, version string) (string, string, string, string) {
	messengerContractAddressURL, messengerDeployerAddressURL, messengerDeployerTxURL, registryBytecodeURL := icm.GetReleaseURLs(version)

	messengerContractAddressPath := filepath.Join(binDir, filepath.Base(messengerContractAddressURL))
	messengerDeployerAddressPath := filepath.Join(binDir, filepath.Base(messengerDeployerAddressURL))
	messengerDeployerTxPath := filepath.Join(binDir, filepath.Base(messengerDeployerTxURL))
	registryBytecodePath := filepath.Join(binDir, filepath.Base(registryBytecodeURL))

	return messengerContractAddressPath, messengerDeployerAddressPath, messengerDeployerTxPath, registryBytecodePath
}

// GetDeployer returns an ICM deployer with assets loaded either from cache or by downloading.
// It manages local file caching of ICM deployment assets.
func GetDeployer(
	icmInstallDir string,
	version string,
) (*icm.Deployer, error) {
	deployer := &icm.Deployer{}

	binDir := filepath.Join(icmInstallDir, version)
	messengerContractAddressPath, messengerDeployerAddressPath, messengerDeployerTxPath, registryBytecodePath := getAssetPaths(binDir, version)

	// Check if all files exist locally
	if utils.FileExists(messengerContractAddressPath) &&
		utils.FileExists(messengerDeployerAddressPath) &&
		utils.FileExists(messengerDeployerTxPath) &&
		utils.FileExists(registryBytecodePath) {
		// Load from local cache
		if err := deployer.LoadFromFiles(
			messengerContractAddressPath,
			messengerDeployerAddressPath,
			messengerDeployerTxPath,
			registryBytecodePath,
		); err != nil {
			return nil, err
		}
		return deployer, nil
	}

	// Download from release
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	if err := deployer.LoadFromRelease(version, token); err != nil {
		return nil, err
	}

	// Cache to disk
	messengerContractAddress, messengerDeployerAddress, messengerDeployerTx, registryBytecode := deployer.Get()

	if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(messengerContractAddressPath, []byte(messengerContractAddress), constants.WriteReadReadPerms); err != nil {
		return nil, err
	}
	if err := os.WriteFile(messengerDeployerAddressPath, []byte(messengerDeployerAddress), constants.WriteReadReadPerms); err != nil {
		return nil, err
	}
	if err := os.WriteFile(messengerDeployerTxPath, messengerDeployerTx, constants.WriteReadReadPerms); err != nil {
		return nil, err
	}
	if err := os.WriteFile(registryBytecodePath, registryBytecode, constants.WriteReadReadPerms); err != nil {
		return nil, err
	}

	return deployer, nil
}

// GetDeployerFromPaths returns an ICM deployer loaded from custom file paths.
func GetDeployerFromPaths(
	messengerContractAddressPath string,
	messengerDeployerAddressPath string,
	messengerDeployerTxPath string,
	registryBytecodePath string,
) (*icm.Deployer, error) {
	deployer := &icm.Deployer{}
	if err := deployer.LoadFromFiles(
		messengerContractAddressPath,
		messengerDeployerAddressPath,
		messengerDeployerTxPath,
		registryBytecodePath,
	); err != nil {
		return nil, err
	}
	return deployer, nil
}

func SetProposerVM(
	app *application.Avalanche,
	network models.Network,
	blockchainID string,
	fundedKeyName string,
) error {
	var (
		err error
		k   *key.SoftKey
	)
	if fundedKeyName == "" {
		if k, err = key.LoadEwoq(network.ID); err != nil {
			return err
		}
	} else {
		k, err = key.LoadSoft(network.ID, app.GetKeyPath(fundedKeyName))
		if err != nil {
			return err
		}
	}
	privKeyStr := k.PrivKeyHex()
	wsEndpoint := network.BlockchainWSEndpoint(blockchainID)
	client, err := evm.GetClient(wsEndpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.SetupProposerVM(privKeyStr)
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
	ti.Version = icm.DefaultVersion
	deployer, err := GetDeployer(
		app.GetICMContractsBinDir(),
		ti.Version,
	)
	if err != nil {
		return nil, err
	}
	_, ti.MessengerDeployerAddress, _, _ = deployer.Get()
	return &ti, nil
}

// Deploy wraps the SDK deployer's Deploy method to match CLI interface.
// Returns: alreadyDeployed, messengerAddress, registryAddress, error
func Deploy(
	deployer *icm.Deployer,
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
		messengerAddress, err = deployer.DeployMessenger(rpcURL, privateKey)
		switch {
		case err == icm.ErrMessengerAlreadyDeployed:
			alreadyDeployed = true
			ux.Logger.PrintToUser("ICM Messenger has already been deployed to %s", subnetName)
		case err != nil:
			return false, "", "", err
		default:
			ux.Logger.PrintToUser("ICM Messenger successfully deployed to %s (%s)", subnetName, messengerAddress)
		}
	}

	if deployRegistry {
		if !deployMessenger || !alreadyDeployed || forceRegistryDeploy {
			registryAddress, err = deployer.DeployRegistry(rpcURL, privateKey)
			if err != nil {
				return alreadyDeployed, messengerAddress, "", err
			}
			ux.Logger.PrintToUser("ICM Registry successfully deployed to %s (%s)", subnetName, registryAddress)
		}
	}

	return alreadyDeployed, messengerAddress, registryAddress, nil
}

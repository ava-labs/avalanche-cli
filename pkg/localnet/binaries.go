// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

// SetupAvalancheGoBinary:
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func SetupAvalancheGoBinary(
	app *application.Avalanche,
	avalancheGoVersion string,
	avalancheGoBinaryPath string,
) (string, error) {
	if avalancheGoBinaryPath == "" {
		_, avalancheGoDir, err := binutils.SetupAvalanchego(app, avalancheGoVersion)
		if err != nil {
			return "", fmt.Errorf("failed setting up avalanchego binary: %w", err)
		}
		avalancheGoBinaryPath = filepath.Join(avalancheGoDir, "avalanchego")
	}
	if !utils.IsExecutable(avalancheGoBinaryPath) {
		return "", fmt.Errorf("avalancheGo binary %s does not exist", avalancheGoBinaryPath)
	}
	return avalancheGoBinaryPath, nil
}

// SetupVMBinary ensures a binary for [blockchainName]'s VM is locally available,
// and provides a path to it
func SetupVMBinary(
	app *application.Avalanche,
	blockchainName string,
) (string, error) {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return "", err
	}
	var binaryPath string
	switch sc.VM {
	case models.SubnetEvm:
		_, binaryPath, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return "", fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.CustomVM:
		binaryPath = binutils.SetupCustomBin(app, blockchainName)
	default:
		return "", fmt.Errorf("unknown vm: %s", sc.VM)
	}
	return binaryPath, nil
}

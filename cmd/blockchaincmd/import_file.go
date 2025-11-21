// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
)

// avalanche blockchain import file
func newImportFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file [blockchainPath]",
		Short: "Import an existing blockchain config",
		RunE:  importFile,
		Args:  cobrautils.MaximumNArgs(1),
		Long: `The blockchain import file command will import a blockchain configuration from a file.

You can optionally provide the path as a command-line argument.
Alternatively, running the command without any arguments triggers an interactive wizard.
By default, an imported blockchain doesn't overwrite an existing blockchain with the same name.
To allow overwrites, provide the --force flag.`,
	}
	cmd.Flags().BoolVarP(
		&overwriteImport,
		"force",
		"f",
		false,
		"overwrite the existing configuration if one exists",
	)
	return cmd
}

func importFile(_ *cobra.Command, args []string) error {
	var (
		importPath string
		err        error
	)
	if len(args) == 1 {
		importPath = args[0]
	}

	if importPath == "" {
		promptStr := "Select the file to import your blockchain from"
		importPath, err = app.Prompt.CaptureExistingFilepath(promptStr)
		if err != nil {
			return err
		}
	}

	importFileBytes, err := os.ReadFile(importPath)
	if err != nil {
		return err
	}

	importable := models.Exportable{}
	err = json.Unmarshal(importFileBytes, &importable)
	if err != nil {
		return err
	}

	blockchainName := importable.Sidecar.Name
	if blockchainName == "" {
		return errors.New("export data is malformed: missing blockchain name")
	}

	if app.GenesisExists(blockchainName) && !overwriteImport {
		return errors.New("blockchain already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if importable.Sidecar.VM == models.CustomVM {
		if importable.Sidecar.CustomVMRepoURL == "" {
			return fmt.Errorf("repository url must be defined for custom vm import")
		}
		if importable.Sidecar.CustomVMBranch == "" {
			return fmt.Errorf("repository branch must be defined for custom vm import")
		}
		if importable.Sidecar.CustomVMBuildScript == "" {
			return fmt.Errorf("build script must be defined for custom vm import")
		}

		if err := vm.BuildCustomVM(app, &importable.Sidecar); err != nil {
			return err
		}

		vmPath := app.GetCustomVMPath(blockchainName)
		rpcVersion, err := vm.GetVMBinaryProtocolVersion(vmPath)
		if err != nil {
			return fmt.Errorf("unable to get custom binary RPC version: %w", err)
		}
		if rpcVersion != importable.Sidecar.RPCVersion {
			return fmt.Errorf("RPC version mismatch between sidecar and vm binary (%d vs %d)", importable.Sidecar.RPCVersion, rpcVersion)
		}
	}

	if err := app.WriteGenesisFile(blockchainName, importable.Genesis); err != nil {
		return err
	}

	if importable.NodeConfig != nil {
		if err := app.WriteAvagoNodeConfigFile(blockchainName, importable.NodeConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetAvagoNodeConfigPath(blockchainName))
	}

	if importable.ChainConfig != nil {
		if err := app.WriteChainConfigFile(blockchainName, importable.ChainConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetChainConfigPath(blockchainName))
	}

	if importable.SubnetConfig != nil {
		if err := app.WriteAvagoSubnetConfigFile(blockchainName, importable.SubnetConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetAvagoSubnetConfigPath(blockchainName))
	}

	if importable.NetworkUpgrades != nil {
		if err := app.WriteNetworkUpgradesFile(blockchainName, importable.NetworkUpgrades); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetUpgradeBytesFilepath(blockchainName))
	}

	if err := app.CreateSidecar(&importable.Sidecar); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Blockchain imported successfully")

	return nil
}

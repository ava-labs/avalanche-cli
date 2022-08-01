// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var overwriteImport bool

// avalanche subnet import
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import [subnetPath]",
		Short:        "Import an existing subnet config",
		RunE:         importSubnet,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Long: `The subnet import command accepts an exported subnet config file.

By default, an imported subnet will not overwrite an existing subnet
with the same name. To allow overwrites, provide the --force flag.`,
	}
	cmd.Flags().BoolVarP(&overwriteImport, "force", "f", false, "overwrite the existing configuration if one exists")
	return cmd
}

func importSubnet(cmd *cobra.Command, args []string) error {
	importPath := args[0]

	importFileBytes, err := os.ReadFile(importPath)
	if err != nil {
		return err
	}

	importable := models.Exportable{}
	err = json.Unmarshal(importFileBytes, &importable)
	if err != nil {
		return err
	}

	subnetName := importable.Sidecar.Name
	if subnetName == "" {
		return errors.New("export data is malformed: missing subnet name")
	}

	if app.GenesisExists(subnetName) && !overwriteImport {
		return errors.New("subnet already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	err = app.WriteGenesisFile(subnetName, importable.Genesis)
	if err != nil {
		return err
	}

	err = app.CreateSidecar(&importable.Sidecar)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Subnet imported successfully")

	return nil
}

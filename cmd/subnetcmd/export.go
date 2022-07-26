// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"
)

var exportOutput string

// avalanche subnet list
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "export [subnetName]",
		Short:        "Export deployment details",
		Long:         `The subnet export command prints the details of an existing subnet deploy.`,
		RunE:         exportSubnet,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	cmd.Flags().StringVarP(
		&exportOutput,
		"output",
		"o",
		"",
		"write the export data to the provided file path",
	)

	return cmd
}

func exportSubnet(cmd *cobra.Command, args []string) error {
	var err error
	if exportOutput == "" {
		pathPrompt := "Enter file path to write export data to"
		exportOutput, err = app.Prompt.CaptureString(pathPrompt)
		if err != nil {
			return err
		}
	}

	subnetName := args[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	gen, err := app.LoadRawGenesis(subnetName)
	if err != nil {
		return err
	}

	exportData := models.Exportable{
		Sidecar: sc,
		Genesis: gen,
	}

	exportBytes, err := json.Marshal(exportData)
	if err != nil {
		return err
	}
	return os.WriteFile(exportOutput, exportBytes, application.WriteReadReadPerms)
}

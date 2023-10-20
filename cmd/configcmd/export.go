// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

var (
	exportOutput        string
)
// avalanche transaction sign
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export avalanche-cli configuration",
		Long: `The configuration export command write the avalanche-cli settings to a file.

The command prompts for an output path. You can also provide one with
the --output flag.`,
		RunE:         exportConfig,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
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

func exportConfig(_ *cobra.Command, args []string) error {
	var err error
	if exportOutput == "" {
		pathPrompt := "Enter file path to write export data to"
		exportOutput, err = app.Prompt.CaptureString(pathPrompt)
		if err != nil {
			return err
		}
	}
	config, err := app.LoadConfig("")
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(&config)
	if err != nil {
		return err
	}
	return app.WriteConfigFile(jsonBytes, exportOutput)
}

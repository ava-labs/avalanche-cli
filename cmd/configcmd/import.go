// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// avalanche transaction sign
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [fileName]",
		Short: "Import avalanche-cli configuration",
		Long: `The configuration import command imports the avalanche-cli settings.

The command prompts for an input path if fileName is not provided.`,
		RunE:         importConfig,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
	}
	return cmd
}

func importConfig(_ *cobra.Command, args []string) error {
	var err error
	var importPath string
	if len(args) == 1 {
		importPath = args[0]
	} else {
		promptStr := "Select the file to import your configuration"
		importPath, err = app.Prompt.CaptureExistingFilepath(promptStr)
		if err != nil {
			return err
		}
	}
	config, err := app.LoadConfig(importPath)
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(&config)
	if err != nil {
		return err
	}
	return app.WriteConfigFile(jsonBytes, "")
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// avalanche subnet list
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all created signing keys",
		Long: `The key list command prints the names of all created signing
keys.`,
		RunE:         listKeys,
		SilenceUsage: true,
	}
}

func listKeys(cmd *cobra.Command, args []string) error {
	header := []string{"Key Name"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)

	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return err
	}

	rows := [][]string{}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			filename := f.Name()
			rows = append(rows, []string{filename[:len(filename)-len(constants.KeySuffix)]})
		}
	}
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

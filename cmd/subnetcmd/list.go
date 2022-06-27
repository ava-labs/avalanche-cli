// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// avalanche subnet list
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all created subnet configurations",
	Long: `The subnet list command prints the names of all created subnet
configurations.`,
	RunE:         listGenesis,
	SilenceUsage: true,
}

type subnetMatrix [][]string

func (c subnetMatrix) Len() int      { return len(c) }
func (c subnetMatrix) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Compare strings by first key of the sub-slice
func (c subnetMatrix) Less(i, j int) bool { return strings.Compare(c[i][0], c[j][0]) == -1 }

func listGenesis(cmd *cobra.Command, args []string) error {
	header := []string{"subnet", "chain", "chain ID", "type", "deployed"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetRowLine(true)

	files, err := os.ReadDir((*app).GetBaseDir())
	if err != nil {
		return err
	}

	rows := subnetMatrix{}

	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		(*app).Log.Warn("could not get connection to server: %w", err)
	}
	if cli != nil {
		ctx := binutils.GetAsyncContext()
		resp, err := cli.Status(ctx)
		if err != nil {
			(*app).Log.Warn("failed to query server for status: %w", err)
		}

		if resp != nil {
			for _, vm := range resp.GetClusterInfo().CustomVms {
				deployedNames[vm.VmName] = struct{}{}
			}
		}
	}

	for _, f := range files {
		if strings.Contains(f.Name(), constants.SidecarSuffix) {
			carName := strings.TrimSuffix(f.Name(), constants.SidecarSuffix)
			// read in sidecar file
			sc, err := (*app).LoadSidecar(carName)
			if err != nil {
				return err
			}

			chainID := sc.ChainID
			// for older sidecars, check in genesis if sidecar has
			// no chainID set
			if chainID == "" {
				sc, err := (*app).LoadEvmGenesis(carName)
				// ignore the error in this case: just leave it to ""
				if err == nil {
					chainID = sc.Config.ChainID.String()
				}
			}

			deployed := "No"
			if _, ok := deployedNames[sc.Subnet]; ok {
				deployed = "Yes"
			}
			rows = append(rows, []string{sc.Subnet, sc.Name, chainID, string(sc.VM), deployed})
		}
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

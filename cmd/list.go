// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
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
	header := []string{"subnet", "chain", "type", "deployed"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetRowLine(true)

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return err
	}

	rows := subnetMatrix{}

	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		log.Warn("could not get connection to server: %w", err)
	}
	ctx := binutils.GetAsyncContext()
	resp, err := cli.Status(ctx)
	if err != nil {
		log.Warn("failed to query server for status: %w", err)
	}

	deployedNames := map[string]struct{}{}
	if resp != nil {
		for _, vm := range resp.GetClusterInfo().CustomVms {
			deployedNames[vm.VmName] = struct{}{}
		}
	}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			sc, err := loadSidecar(strings.TrimSuffix(f.Name(), sidecar_suffix))
			if err != nil {
				return err
			}

			deployed := "No"
			if _, ok := deployedNames[sc.Subnet]; ok {
				deployed = "Yes"
			}
			rows = append(rows, []string{sc.Subnet, sc.Name, string(sc.Vm), deployed})
		}
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

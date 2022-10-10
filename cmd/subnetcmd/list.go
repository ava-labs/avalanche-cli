// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// avalanche subnet list
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all created subnet configurations",
		Long: `The subnet list command prints the names of all created subnet
configurations.`,
		RunE:         listSubnets,
		SilenceUsage: true,
	}
}

type subnetMatrix [][]string

func (c subnetMatrix) Len() int      { return len(c) }
func (c subnetMatrix) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Compare strings by first key of the sub-slice
func (c subnetMatrix) Less(i, j int) bool { return strings.Compare(c[i][0], c[j][0]) == -1 }

func listSubnets(cmd *cobra.Command, args []string) error {
	header := []string{"subnet", "chain", "chain ID", "type", "vm version", "from repo", "", "deployed", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	files, err := os.ReadDir(app.GetBaseDir())
	if err != nil {
		return err
	}

	rows := subnetMatrix{}
	// append a second "header" row for the networks
	rows = append(rows, []string{"", "", "", "", "", "", "Local", "Fuji", "Mainnet"})

	deployedNames, err := subnet.GetLocallyDeployedSubnets(app)
	if err != nil {
		// if the server can not be contacted, or there is a problem with the query,
		// DO NOT FAIL, just print No for deployed status
		app.Log.Warn("problem contacting server to get deployed subnets")
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.SidecarSuffix) {
			carName := strings.TrimSuffix(f.Name(), constants.SidecarSuffix)
			// read in sidecar file
			sc, err := app.LoadSidecar(carName)
			if err != nil {
				return err
			}

			chainID := sc.ChainID
			// for older sidecars, check in genesis if sidecar has
			// no chainID set
			if chainID == "" {
				sc, err := app.LoadEvmGenesis(carName)
				// ignore the error in this case: just leave it to ""
				if err == nil {
					chainID = sc.Config.ChainID.String()
				}
			}

			deployedLocal := "No"
			if _, ok := deployedNames[sc.Subnet]; ok {
				deployedLocal = "Yes"
			}
			deployedFuji := "No"
			if _, ok := sc.Networks[models.Fuji.String()]; ok {
				if sc.Networks[models.Fuji.String()].SubnetID != ids.Empty {
					deployedFuji = "Yes"
				}
			}
			deployedMain := "N/A"
			rows = append(rows, []string{
				sc.Subnet,
				sc.Name,
				chainID,
				string(sc.VM),
				sc.VMVersion,
				strconv.FormatBool(sc.ImportedFromAPM),
				deployedLocal,
				deployedFuji,
				deployedMain,
			})
		}
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

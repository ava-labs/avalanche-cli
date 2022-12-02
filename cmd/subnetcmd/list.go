// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// avalanche subnet list
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List all created subnet configurations",
		Long:         `The subnet list command prints the names of all created Subnet configurations.`,
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
	header := []string{"subnet", "chain", "chain ID", "type", "from repo", "", "deployed", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	subnets, err := os.ReadDir(filepath.Join(app.GetBaseDir(), constants.SubnetDir))
	if err != nil {
		return err
	}

	rows := subnetMatrix{}
	// append a second "header" row for the networks
	rows = append(rows, []string{"", "", "", "", "", "Local", "Fuji", "Mainnet"})

	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		app.Log.Warn("could not get connection to server", zap.Error(err))
	}
	if cli != nil {
		ctx := binutils.GetAsyncContext()
		resp, err := cli.Status(ctx)
		if err != nil {
			app.Log.Warn("failed to query server for status", zap.Error(err))
		}

		if resp != nil {
			for _, chain := range resp.GetClusterInfo().CustomChains {
				deployedNames[chain.ChainName] = struct{}{}
			}
		}
	}

	for _, s := range subnets {
		// this shouldn't happen but let's be safe
		if !s.IsDir() {
			continue
		}
		subnetDir := filepath.Join(app.GetSubnetDir(), s.Name())
		files, err := os.ReadDir(subnetDir)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.Name() == constants.SidecarFileName {
				carName := s.Name()
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

				deployedLocal := constants.NoLabel
				if _, ok := deployedNames[sc.Subnet]; ok {
					deployedLocal = constants.YesLabel
				}
				deployedFuji := constants.NoLabel
				if _, ok := sc.Networks[models.Fuji.String()]; ok {
					if sc.Networks[models.Fuji.String()].SubnetID != ids.Empty {
						deployedFuji = constants.YesLabel
					}
				}
				deployedMain := constants.NoLabel
				if _, ok := sc.Networks[models.Mainnet.String()]; ok {
					if sc.Networks[models.Mainnet.String()].SubnetID != ids.Empty {
						deployedMain = constants.YesLabel
					}
				}
				rows = append(rows, []string{
					sc.Subnet,
					sc.Name,
					chainID,
					string(sc.VM),
					strconv.FormatBool(sc.ImportedFromAPM),
					deployedLocal,
					deployedFuji,
					deployedMain,
				})
			}
		}
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

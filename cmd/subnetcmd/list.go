// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var deployed bool

// avalanche subnet list
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List all created Subnet configurations",
		Long:         `The Subnet list command prints the names of all created Subnet configurations.`,
		RunE:         listSubnets,
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&deployed, "deployed", false, "show additional deploy information")
	return cmd
}

type subnetMatrix [][]string

func (c subnetMatrix) Len() int      { return len(c) }
func (c subnetMatrix) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Compare strings by first key of the sub-slice
func (c subnetMatrix) Less(i, j int) bool { return strings.Compare(c[i][0], c[j][0]) == -1 }

func listSubnets(cmd *cobra.Command, args []string) error {
	if deployed {
		return listDeployInfo(cmd, args)
	}
	header := []string{"subnet", "chain", "chainID", "vmID", "type", "from repo"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	rows := subnetMatrix{}

	cars, err := getSidecars(app)
	if err != nil {
		return err
	}
	for _, sc := range cars {
		chainID := sc.ChainID
		// for older sidecars, check in genesis if sidecar has
		// no chainID set
		if chainID == "" {
			sc, err := app.LoadEvmGenesis(sc.Name)
			// ignore the error in this case: just leave it to ""
			if err == nil {
				chainID = sc.Config.ChainID.String()
			}
		}

		vmID := sc.ImportedVMID
		if vmID == "" {
			id, err := utils.VMID(sc.Name)
			if err != nil {
				vmID = constants.NotAvailableLabel
			} else {
				vmID = id.String()
			}
		}
		rows = append(rows, []string{
			sc.Subnet,
			sc.Name,
			chainID,
			vmID,
			string(sc.VM),
			strconv.FormatBool(sc.ImportedFromAPM),
		})
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
	return nil
}

func getSidecars(app *application.Avalanche) ([]*models.Sidecar, error) {
	subnets, err := os.ReadDir(filepath.Join(app.GetBaseDir(), constants.SubnetDir))
	if err != nil {
		return nil, err
	}

	var cars []*models.Sidecar
	for _, s := range subnets {
		// this shouldn't happen but let's be safe
		if !s.IsDir() {
			continue
		}
		subnetDir := filepath.Join(app.GetSubnetDir(), s.Name())
		files, err := os.ReadDir(subnetDir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Name() == constants.SidecarFileName {
				carName := s.Name()
				// read in sidecar file
				sc, err := app.LoadSidecar(carName)
				if err != nil {
					return nil, err
				}
				cars = append(cars, &sc)
			}
		}
	}
	return cars, nil
}

func listDeployInfo(cmd *cobra.Command, args []string) error {
	header := []string{"subnet", "chain", "vm ID", "Local Network", "Fuji (testnet)", "Mainnet"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 2, 3, 4})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)

	rows := subnetMatrix{}

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
	cars, err := getSidecars(app)
	if err != nil {
		return err
	}

	fujiKey := models.Fuji.String()
	mainKey := models.Mainnet.String()

	singleLine := true

	for _, sc := range cars {
		netToID := map[string][]string{}
		deployedLocal := constants.NoLabel
		if _, ok := deployedNames[sc.Subnet]; ok {
			deployedLocal = constants.YesLabel
		}
		if _, ok := sc.Networks[fujiKey]; ok {
			if sc.Networks[fujiKey].SubnetID != ids.Empty {
				netToID[fujiKey] = []string{
					constants.SubnetIDLabel + sc.Networks[fujiKey].SubnetID.String(),
					constants.BlockchainIDLabel + sc.Networks[fujiKey].BlockchainID.String(),
				}
				singleLine = false
			}
		} else {
			netToID[fujiKey] = []string{constants.NoLabel, constants.NoLabel}
		}
		if _, ok := sc.Networks[mainKey]; ok {
			if sc.Networks[mainKey].SubnetID != ids.Empty {
				netToID[mainKey] = []string{
					constants.SubnetIDLabel + sc.Networks[mainKey].SubnetID.String(),
					constants.BlockchainIDLabel + sc.Networks[mainKey].BlockchainID.String(),
				}
				singleLine = false
			}
		} else {
			netToID[mainKey] = []string{constants.NoLabel, constants.NoLabel}
		}
		vmID := sc.ImportedVMID
		if vmID == "" {
			id, err := utils.VMID(sc.Name)
			if err != nil {
				vmID = constants.NotAvailableLabel
			} else {
				vmID = id.String()
			}
		}

		rows = append(rows, []string{
			sc.Subnet,
			sc.Name,
			vmID,
			deployedLocal,
			netToID[fujiKey][0],
			netToID[mainKey][0],
		})

		if !singleLine {
			rows = append(rows, []string{
				sc.Subnet,
				sc.Name,
				vmID,
				deployedLocal,
				netToID[fujiKey][1],
				netToID[mainKey][1],
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

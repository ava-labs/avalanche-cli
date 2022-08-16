// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm"
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
	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return err
	}

	keyPaths := make([]string, len(files))

	for i, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keyPaths[i] = filepath.Join(app.GetKeyDir(), f.Name())
		}
	}
	return printAddresses(keyPaths)
}

func printAddresses(keyPaths []string) error {
	header := []string{"Key Name", "Chain", "Address", "Balance", "Network"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)

	supportedNetworks := map[string]uint32{
		models.Local.String(): 0,
		models.Fuji.String():  avago_constants.FujiID,
		/*
			Not enabled yet
			models.Mainnet.String(): avago_constants.MainnetID,
		*/
	}

	rootCtx := context.Background()
	fujiPClient := platformvm.NewClient(constants.FujiAPIEndpoint)

	for _, keyPath := range keyPaths {
		cAdded := false
		keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
		for net, id := range supportedNetworks {
			sk, err := key.LoadSoft(id, keyPath)
			if err != nil {
				return err
			}
			if !cAdded {
				strC := sk.C()
				table.Append([]string{keyName, "C-Chain (Ethereum hex format)", strC, "0", "All"})
			}
			cAdded = true

			strP := sk.P()
			for _, p := range strP {
				pID, err := address.ParseToID(p)
				if err != nil {
					return err
				}
				balanceStr := ""
				if net == models.Fuji.String() {
					ctx, cancel := context.WithTimeout(rootCtx, constants.RequestTimeout)
					resp, err := fujiPClient.GetBalance(ctx, []ids.ShortID{pID})
					cancel()
					if err != nil {
						return err
					}
					balanceStr = fmt.Sprintf("%d", uint64(resp.Balance))
				}
				table.Append([]string{keyName, "P-Chain (Bech32 format)", p, balanceStr, net})
			}
		}
	}

	table.Render()
	return nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const allNetworksFlag = "all-networks"

var allNetworks bool

// avalanche subnet list
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all created signing keys",
		Long: `The key list command prints the names of all created signing
keys.`,
		RunE:         listKeys,
		SilenceUsage: true,
	}
	cmd.Flags().BoolVarP(
		&allNetworks,
		allNetworksFlag,
		"a",
		false,
		"list also local network addresses",
	)
	return cmd
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
		models.Fuji.String(): avago_constants.FujiID,
		/*
			Not enabled yet
			models.Mainnet.String(): avago_constants.MainnetID,
		*/
	}

	if allNetworks {
		supportedNetworks[models.Local.String()] = 0
	}

	ctx := context.Background()
	fujiPClient := platformvm.NewClient(constants.FujiAPIEndpoint)
	fujiCClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", constants.FujiAPIEndpoint, "C"))
	if err != nil {
		return err
	}

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
				balanceStr, err := getCChainBalanceStr(ctx, fujiCClient, strC)
				if err != nil {
					return err
				}
				table.Append([]string{keyName, "C-Chain (Ethereum hex format)", strC, balanceStr, "All"})
			}
			cAdded = true

			strP := sk.P()
			for _, p := range strP {
				balanceStr := "0"
				if net == models.Fuji.String() {
					var err error
					balanceStr, err = getPChainBalanceStr(ctx, fujiPClient, p)
					if err != nil {
						return err
					}
				}
				table.Append([]string{keyName, "P-Chain (Bech32 format)", p, balanceStr, net})
			}
		}
	}

	table.Render()
	return nil
}

func getCChainBalanceStr(ctx context.Context, cClient ethclient.Client, addrStr string) (string, error) {
	addr := common.HexToAddress(addrStr)
	ctx, cancel := context.WithTimeout(ctx, constants.RequestTimeout)
	balance, err := cClient.BalanceAt(ctx, addr, nil)
	cancel()
	if err != nil {
		return "", err
	}
	if balance.Cmp(big.NewInt(0)) == 0 {
		return "0", nil
	}
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	return fmt.Sprintf("%.9f", float64(balance.Uint64())/float64(units.Avax)), nil
}

func getPChainBalanceStr(ctx context.Context, pClient platformvm.Client, addr string) (string, error) {
	pID, err := address.ParseToID(addr)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, constants.RequestTimeout)
	resp, err := pClient.GetBalance(ctx, []ids.ShortID{pID})
	cancel()
	if err != nil {
		return "", err
	}
	if resp.Balance == 0 {
		return "0", nil
	}
	return fmt.Sprintf("%.9f", float64(resp.Balance)/float64(units.Avax)), nil
}

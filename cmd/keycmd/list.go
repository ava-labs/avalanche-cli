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
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ledger "github.com/ava-labs/avalanche-ledger-go"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	allNetworksFlag   = "all-networks"
	ledgerIndicesFlag = "ledger"
)

var (
	allNetworks   bool
	ledgerIndices []uint
)

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
	cmd.Flags().UintSliceVarP(
		&ledgerIndices,
		ledgerIndicesFlag,
		"l",
		[]uint{},
		"list ledger addresses for the given indices (if two are given, will consider they as a range specification)",
	)
	return cmd
}

func listKeys(cmd *cobra.Command, args []string) error {
	if len(ledgerIndices) > 0 {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return err
		}
		// ask for addresses here to print user msg for ledger interaction
		ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
		maxIndex := math.Max(0, ledgerIndices...)
		toDerive := int(maxIndex + 1)
		addresses, err := ledgerDevice.Addresses(toDerive)
		if err != nil {
			return err
		}
		if len(addresses) != toDerive {
			return fmt.Errorf("derived addresses %d differ from expected %d", len(addresses), toDerive)
		}
		network := models.Local
		networkID, err := network.NetworkID()
		if err != nil {
			return err
		}
		for _, index := range ledgerIndices {
			addr := addresses[index]
			addrStr, err := address.Format("P", key.GetHRP(networkID), addr[:])
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("Ledger address: %s", addrStr)))
		}
	}
	return nil

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

	addrInfos := []addressInfo{}
	printAddrInfos(addrInfos)

	return nil
}

type addressInfo struct {
	name    string
	kind    string
	chain   string
	address string
	balance string
	network string
}

func printAddrInfos(addrInfos []addressInfo) {
	header := []string{"Name", "Kind", "Chain", "Address", "Balance", "Network"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	for _, addrInfo := range addrInfos {
		table.Append([]string{
			addrInfo.name,
			addrInfo.kind,
			addrInfo.chain,
			addrInfo.address,
			addrInfo.balance,
			addrInfo.network})
	}
	table.Render()
}

func getAddrInfos(keyPaths []string) error {
	header := []string{"Key Name", "Chain", "Address", "Balance", "Network"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})

	supportedNetworks := map[string]uint32{
		models.Fuji.String():    avago_constants.FujiID,
		models.Mainnet.String(): avago_constants.MainnetID,
	}

	if allNetworks {
		supportedNetworks[models.Local.String()] = 0
	}

	// get clients
	ctx := context.Background()
	fujiPClient := platformvm.NewClient(constants.FujiAPIEndpoint)
	fujiCClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", constants.FujiAPIEndpoint, "C"))
	if err != nil {
		return err
	}
	mainnetPClient := platformvm.NewClient(constants.MainnetAPIEndpoint)
	mainnetCClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", constants.MainnetAPIEndpoint, "C"))
	if err != nil {
		return err
	}
	_ = mainnetPClient
	_ = mainnetCClient

	for _, keyPath := range keyPaths {
		keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
		for net, id := range supportedNetworks {
			sk, err := key.LoadSoft(id, keyPath)
			if err != nil {
				return err
			}

			strC := sk.C()
			balanceStr := ""
			if net == models.Fuji.String() {
				balanceStr, err = getCChainBalanceStr(ctx, fujiCClient, strC)
				if err != nil {
					return err
				}
			}
			table.Append([]string{keyName, "C-Chain (Ethereum hex format)", strC, balanceStr, net})

			strP := sk.P()
			for _, p := range strP {
				balanceStr := ""
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
	// convert to nAvax
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	if balance.Cmp(big.NewInt(0)) == 0 {
		return "0", nil
	}
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

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
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	localFlag         = "local"
	fujiFlag          = "fuji"
	testnetFlag       = "testnet"
	mainnetFlag       = "mainnet"
	allFlag           = "all-networks"
    cchainFlag        = "cchain"
	ledgerIndicesFlag = "ledger"
)

var (
	local         bool
	testnet       bool
	mainnet       bool
	all           bool
    cchain        bool
	ledgerIndices []uint
)

// avalanche subnet list
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored signing keys or ledger addresses",
		Long: `The key list command prints information for all stored signing
keys or for the ledger addresses associated to certain indices.`,
		RunE:         listKeys,
		SilenceUsage: true,
	}
	cmd.Flags().BoolVarP(
		&local,
		localFlag,
		"l",
		false,
		"list local network addresses",
	)
	cmd.Flags().BoolVarP(
		&testnet,
		fujiFlag,
		"f",
		false,
		"list testnet (fuji) network addresses",
	)
	cmd.Flags().BoolVarP(
		&testnet,
		testnetFlag,
		"t",
		false,
		"list testnet (fuji) network addresses",
	)
	cmd.Flags().BoolVarP(
		&mainnet,
		mainnetFlag,
		"m",
		true,
		"list mainnet network addresses",
	)
	cmd.Flags().BoolVarP(
		&all,
		allFlag,
		"a",
		false,
		"list all network addresses",
	)
	cmd.Flags().BoolVarP(
		&cchain,
		cchainFlag,
		"c",
		true,
		"list C-Chain addresses",
	)
	cmd.Flags().UintSliceVarP(
		&ledgerIndices,
		ledgerIndicesFlag,
		"g",
		[]uint{},
		"list ledger addresses for the given indices",
	)
	return cmd
}

func getClients(networks []models.Network, cchain bool) (
    map[models.Network]platformvm.Client,
    map[models.Network]ethclient.Client,
    error,
) {
    apiEndpoints := map[models.Network]string{
		models.Fuji:    constants.FujiAPIEndpoint,
		models.Mainnet: constants.MainnetAPIEndpoint,
		models.Local:   constants.LocalAPIEndpoint,
    }
    var err error
	pClients := map[models.Network]platformvm.Client{}
	cClients := map[models.Network]ethclient.Client{}
    for _, network := range networks {
        fmt.Println("p", network)
		pClients[network] = platformvm.NewClient(apiEndpoints[network])
        if cchain {
            fmt.Println("c", network)
            cClients[network], err = ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", apiEndpoints[network], "C"))
            if err != nil {
                return nil, nil, err
            }
        }
    }
    return pClients, cClients, nil
}

func listKeys(cmd *cobra.Command, args []string) error {
	addrInfos := []addressInfo{}
	networks := []models.Network{}
	if local || all {
		networks = append(networks, models.Local)
	}
	if testnet || all {
		networks = append(networks, models.Fuji)
	}
	if mainnet || all {
		networks = append(networks, models.Mainnet)
	}
	if len(networks) == 0 {
		return fmt.Errorf("you must specify at least one of --local, --fuji, --testnet, --mainnet")
	}
    queryLedger := len(ledgerIndices) > 0
    if queryLedger {
        cchain = false
    }
    pClients, cClients, err := getClients(networks, cchain)
    if err != nil {
        return err
    }
	if queryLedger {
		addrInfos, err = getLedgerAddrInfos(pClients, ledgerIndices, networks)
		if err != nil {
			return err
		}
	} else {
		addrInfos, err = getStoredKeyInfos(pClients, cClients, networks)
		if err != nil {
			return err
		}
	}
	printAddrInfos(addrInfos)
	return nil
}

func getStoredKeyInfos(
	pClients map[models.Network]platformvm.Client,
	cClients map[models.Network]ethclient.Client,
	networks []models.Network,
    cchain bool,
) ([]addressInfo, error) {
    files, err := os.ReadDir(app.GetKeyDir())
    if err != nil {
        return nil, err
    }
    keyPaths := make([]string, len(files))
    for i, f := range files {
        if strings.HasSuffix(f.Name(), constants.KeySuffix) {
            keyPaths[i] = filepath.Join(app.GetKeyDir(), f.Name())
        }
    }
	addrInfos := []addressInfo{}
    return addrInfos, nil
}

func getStoredKeyInfo(
	pClients map[models.Network]platformvm.Client,
	cClients map[models.Network]ethclient.Client,
	network models.Network,
    ketPath string,
    cchain bool,
) (addressInfo, error) {
	networkID, err := network.NetworkID()
	if err != nil {
		return addressInfo{}, err
	}
    keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
    sk, err := key.LoadSoft(networkID, keyPath)
    if err != nil {
        return addressInfo{}, err
    }
    if cchain {
        cChainAddr := sk.C()
        cChainBalance, err := getCChainBalanceStr(context.Background(), cClients[network], cChainAddr)
        if err != nil {
            // just ignore local network errors
            if network != models.Local {
                return addressInfo{}, err
            }
        }
        return addressInfo{
            kind:    "stored",
            name:    keyName,
            chain:   "C-Chain (Ethereum hex format)",
            address: cChainAddr,
            balance: cChainBalance,
            network: network.String(),
        }, nil
    }
    pChainAddr := sk.P()
	balance, err := getPChainBalanceStr(context.Background(), pClients[network], pChainAddr)
	if err != nil {
		// just ignore local network errors
		if network != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    "stored",
		name:    keyName,
		chain:   "P-Chain (Bech32 format)",
		address: addrStr,
		balance: balance,
		network: network.String(),
	}, nil
}

func getLedgerAddrInfos(
	pClients map[models.Network]platformvm.Client,
	ledgerIndices []uint,
	networks []models.Network,
) ([]addressInfo, error) {
	ledgerDevice, err := ledger.New()
	if err != nil {
		return nil, err
	}
	ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
	maxIndex := math.Max(0, ledgerIndices...)
	toDerive := int(maxIndex + 1)
	addresses, err := ledgerDevice.Addresses(toDerive)
	if err != nil {
		return nil, err
	}
	if len(addresses) != toDerive {
		return nil, fmt.Errorf("derived addresses length %d differs from expected %d", len(addresses), toDerive)
	}
	addrInfos := []addressInfo{}
	for _, index := range ledgerIndices {
		addr := addresses[index]
		for _, network := range networks {
			addrInfo, err := getLedgerAddrInfo(pClients, index, network, addr)
			if err != nil {
				return []addressInfo{}, err
			}
			addrInfos = append(addrInfos, addrInfo)
		}
	}
	return addrInfos, nil
}

func getLedgerAddrInfo(
	pClients map[models.Network]platformvm.Client,
	index uint,
	network models.Network,
	addr ids.ShortID,
) (addressInfo, error) {
	networkID, err := network.NetworkID()
	if err != nil {
		return addressInfo{}, err
	}
	addrStr, err := address.Format("P", key.GetHRP(networkID), addr[:])
	if err != nil {
		return addressInfo{}, err
	}
	balance, err := getPChainBalanceStr(context.Background(), pClients[network], addrStr)
	if err != nil {
		// just ignore local network errors
		if network != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    "ledger",
		name:    fmt.Sprintf("index %d", index),
		chain:   "P-Chain (Bech32 format)",
		address: addrStr,
		balance: balance,
		network: network.String(),
	}, nil
}

type addressInfo struct {
	kind    string
	name    string
	chain   string
	address string
	balance string
	network string
}

func printAddrInfos(addrInfos []addressInfo) {
	header := []string{"Kind", "Name", "Chain", "Address", "Balance", "Network"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1, 2})
	for _, addrInfo := range addrInfos {
		table.Append([]string{
			addrInfo.kind,
			addrInfo.name,
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

	//if allNetworks {
	//	supportedNetworks[models.Local.String()] = 0
	//}

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

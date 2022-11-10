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
		pClients[network] = platformvm.NewClient(apiEndpoints[network])
		if cchain {
			cClients[network], err = ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", apiEndpoints[network], "C"))
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return pClients, cClients, nil
}

type addressInfo struct {
	kind    string
	name    string
	chain   string
	address string
	balance string
	network string
}

func listKeys(cmd *cobra.Command, args []string) error {
	var addrInfos []addressInfo
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
		addrInfos, err = getStoredKeyInfos(pClients, cClients, networks, cchain)
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
	for _, keyPath := range keyPaths {
		keyAddrInfos, err := getStoredKeyInfo(pClients, cClients, networks, keyPath, cchain)
		if err != nil {
			return nil, err
		}
		addrInfos = append(addrInfos, keyAddrInfos...)
	}
	return addrInfos, nil
}

func getStoredKeyInfo(
	pClients map[models.Network]platformvm.Client,
	cClients map[models.Network]ethclient.Client,
	networks []models.Network,
	keyPath string,
	cchain bool,
) ([]addressInfo, error) {
	addrInfos := []addressInfo{}
	for _, network := range networks {
		networkID, err := network.NetworkID()
		if err != nil {
			return nil, err
		}
		keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
		sk, err := key.LoadSoft(networkID, keyPath)
		if err != nil {
			return nil, err
		}
		if cchain {
			cChainAddr := sk.C()
			addrInfo, err := getCChainAddrInfo(cClients, network, cChainAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo)
		}
		pChainAddrs := sk.P()
		for _, pChainAddr := range pChainAddrs {
			addrInfo, err := getPChainAddrInfo(pClients, network, pChainAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo)
		}
	}
	return addrInfos, nil
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
	pChainAddr, err := address.Format("P", key.GetHRP(networkID), addr[:])
	if err != nil {
		return addressInfo{}, err
	}
	return getPChainAddrInfo(
		pClients,
		network,
		pChainAddr,
		"ledger",
		fmt.Sprintf("index %d", index),
	)
}

func getPChainAddrInfo(
	pClients map[models.Network]platformvm.Client,
	network models.Network,
	pChainAddr string,
	kind string,
	name string,
) (addressInfo, error) {
	balance, err := getPChainBalanceStr(context.Background(), pClients[network], pChainAddr)
	if err != nil {
		// just ignore local network errors
		if network != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    kind,
		name:    name,
		chain:   "P-Chain (Bech32 format)",
		address: pChainAddr,
		balance: balance,
		network: network.String(),
	}, nil
}

func getCChainAddrInfo(
	cClients map[models.Network]ethclient.Client,
	network models.Network,
	cChainAddr string,
	kind string,
	name string,
) (addressInfo, error) {
	cChainBalance, err := getCChainBalanceStr(context.Background(), cClients[network], cChainAddr)
	if err != nil {
		// just ignore local network errors
		if network != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    kind,
		name:    name,
		chain:   "C-Chain (Ethereum hex format)",
		address: cChainAddr,
		balance: cChainBalance,
		network: network.String(),
	}, nil
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
			addrInfo.network,
		})
	}
	table.Render()
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

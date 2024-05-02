// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	allFlag           = "all-networks"
	pchainFlag        = "pchain"
	cchainFlag        = "cchain"
	xchainFlag        = "xchain"
	chainsFlag        = "chains"
	ledgerIndicesFlag = "ledger"
	useNanoAvaxFlag   = "use-nano-avax"
)

var (
	globalNetworkFlags          networkoptions.NetworkFlags
	listSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Mainnet, networkoptions.Fuji, networkoptions.Local, networkoptions.Cluster}
	all                         bool
	pchain                      bool
	cchain                      bool
	xchain                      bool
	chains                      string
	useNanoAvax                 bool
	ledgerIndices               []uint
	subnetName                  string
)

// avalanche subnet list
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored signing keys or ledger addresses",
		Long: `The key list command prints information for all stored signing
keys or for the ledger addresses associated to certain indices.`,
		RunE: listKeys,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, listSupportedNetworkOptions)
	cmd.Flags().BoolVarP(
		&all,
		allFlag,
		"a",
		false,
		"list all network addresses",
	)
	cmd.Flags().BoolVar(
		&pchain,
		pchainFlag,
		true,
		"list P-Chain addresses",
	)
	cmd.Flags().BoolVarP(
		&cchain,
		cchainFlag,
		"c",
		true,
		"list C-Chain addresses",
	)
	cmd.Flags().BoolVar(
		&xchain,
		xchainFlag,
		true,
		"list X-Chain addresses",
	)
	cmd.Flags().BoolVarP(
		&useNanoAvax,
		useNanoAvaxFlag,
		"n",
		false,
		"use nano Avax for balances",
	)
	cmd.Flags().UintSliceVarP(
		&ledgerIndices,
		ledgerIndicesFlag,
		"g",
		[]uint{},
		"list ledger addresses for the given indices",
	)
	cmd.Flags().StringVar(
		&subnetName,
		"subnet",
		"",
		"provide balance information for the given subnet (Subnet-Evm based only)",
	)
	cmd.Flags().StringVar(
		&chains,
		chainsFlag,
		"pxc",
		"short way to specify which chains to show information about (p=show p-chain, x=show x-chain, c=show c-chain). defaults to pxc",
	)
	return cmd
}

func getClients(networks []models.Network, pchain bool, cchain bool, xchain bool, subnetName string) (
	map[models.Network]platformvm.Client,
	map[models.Network]avm.Client,
	map[models.Network]ethclient.Client,
	map[models.Network]ethclient.Client,
	error,
) {
	var err error
	xClients := map[models.Network]avm.Client{}
	pClients := map[models.Network]platformvm.Client{}
	cClients := map[models.Network]ethclient.Client{}
	evmClients := map[models.Network]ethclient.Client{}
	for _, network := range networks {
		if pchain {
			pClients[network] = platformvm.NewClient(network.Endpoint)
		}
		if xchain {
			xClients[network] = avm.NewClient(network.Endpoint, "X")
		}
		if cchain {
			cClients[network], err = ethclient.Dial(network.CChainEndpoint())
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		if subnetName != "" {
			_, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
			if err != nil {
				return nil, nil, nil, nil, err
			}
			b, err := subnetcmd.HasSubnetEVMGenesis(subnetName)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if b {
				sc, err := app.LoadSidecar(subnetName)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				chainID := sc.Networks[network.Name()].BlockchainID
				if chainID != ids.Empty {
					evmClients[network], err = ethclient.Dial(network.BlockchainEndpoint(chainID.String()))
					if err != nil {
						return nil, nil, nil, nil, err
					}
				}
			}
		}
	}
	return pClients, xClients, cClients, evmClients, nil
}

type addressInfo struct {
	kind    string
	name    string
	chain   string
	address string
	balance string
	network string
}

func listKeys(*cobra.Command, []string) error {
	var addrInfos []addressInfo
	networks := []models.Network{}
	if globalNetworkFlags.UseLocal || all {
		networks = append(networks, models.NewLocalNetwork())
	}
	if globalNetworkFlags.UseFuji || all {
		networks = append(networks, models.NewFujiNetwork())
	}
	if globalNetworkFlags.UseMainnet || all {
		networks = append(networks, models.NewMainnetNetwork())
	}
	if globalNetworkFlags.ClusterName != "" {
		network, err := app.GetClusterNetwork(globalNetworkFlags.ClusterName)
		if err != nil {
			return err
		}
		networks = append(networks, network)
	}
	if len(networks) == 0 {
		network, err := networkoptions.GetNetworkFromCmdLineFlags(
			app,
			networkoptions.NetworkFlags{},
			false,
			listSupportedNetworkOptions,
			subnetName,
		)
		if err != nil {
			return err
		}
		networks = append(networks, network)
	}
	if !strings.Contains(chains, "p") {
		pchain = false
	}
	if !strings.Contains(chains, "x") {
		xchain = false
	}
	if !strings.Contains(chains, "c") {
		cchain = false
	}
	queryLedger := len(ledgerIndices) > 0
	if queryLedger {
		pchain = true
		cchain = false
		xchain = false
	}
	pClients, xClients, cClients, evmClients, err := getClients(networks, pchain, cchain, xchain, subnetName)
	if err != nil {
		return err
	}
	if queryLedger {
		ledgerIndicesU32 := []uint32{}
		for _, index := range ledgerIndices {
			ledgerIndicesU32 = append(ledgerIndicesU32, uint32(index))
		}
		addrInfos, err = getLedgerIndicesInfo(pClients, ledgerIndicesU32, networks)
		if err != nil {
			return err
		}
	} else {
		addrInfos, err = getStoredKeysInfo(pClients, xClients, cClients, evmClients, networks)
		if err != nil {
			return err
		}
	}
	printAddrInfos(addrInfos)
	return nil
}

func getStoredKeysInfo(
	pClients map[models.Network]platformvm.Client,
	xClients map[models.Network]avm.Client,
	cClients map[models.Network]ethclient.Client,
	evmClients map[models.Network]ethclient.Client,
	networks []models.Network,
) ([]addressInfo, error) {
	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return nil, err
	}
	keyPaths := []string{}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keyPaths = append(keyPaths, filepath.Join(app.GetKeyDir(), f.Name()))
		}
	}
	addrInfos := []addressInfo{}
	for _, keyPath := range keyPaths {
		keyAddrInfos, err := getStoredKeyInfo(pClients, xClients, cClients, evmClients, networks, keyPath)
		if err != nil {
			return nil, err
		}
		addrInfos = append(addrInfos, keyAddrInfos...)
	}
	return addrInfos, nil
}

func getStoredKeyInfo(
	pClients map[models.Network]platformvm.Client,
	xClients map[models.Network]avm.Client,
	cClients map[models.Network]ethclient.Client,
	evmClients map[models.Network]ethclient.Client,
	networks []models.Network,
	keyPath string,
) ([]addressInfo, error) {
	addrInfos := []addressInfo{}
	for _, network := range networks {
		keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
		sk, err := key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return nil, err
		}
		if _, ok := evmClients[network]; ok {
			evmAddr := sk.C()
			addrInfo, err := getEvmBasedChainAddrInfo(subnetName, evmClients, network, evmAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo)
		}
		if _, ok := cClients[network]; ok {
			cChainAddr := sk.C()
			addrInfo, err := getEvmBasedChainAddrInfo("C-Chain", cClients, network, cChainAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo)
		}
		if _, ok := pClients[network]; ok {
			pChainAddrs := sk.P()
			for _, pChainAddr := range pChainAddrs {
				addrInfo, err := getPChainAddrInfo(pClients, network, pChainAddr, "stored", keyName)
				if err != nil {
					return nil, err
				}
				addrInfos = append(addrInfos, addrInfo)
			}
		}
		if _, ok := xClients[network]; ok {
			xChainAddrs := sk.X()
			for _, xChainAddr := range xChainAddrs {
				addrInfo, err := getXChainAddrInfo(xClients, network, xChainAddr, "stored", keyName)
				if err != nil {
					return nil, err
				}
				addrInfos = append(addrInfos, addrInfo)
			}
		}
	}
	return addrInfos, nil
}

func getLedgerIndicesInfo(
	pClients map[models.Network]platformvm.Client,
	ledgerIndices []uint32,
	networks []models.Network,
) ([]addressInfo, error) {
	ledgerDevice, err := ledger.New()
	if err != nil {
		return nil, err
	}
	addresses, err := ledgerDevice.Addresses(ledgerIndices)
	if err != nil {
		return nil, err
	}
	if len(addresses) != len(ledgerIndices) {
		return nil, fmt.Errorf("derived addresses length %d differs from expected %d", len(addresses), len(ledgerIndices))
	}
	addrInfos := []addressInfo{}
	for i, index := range ledgerIndices {
		addr := addresses[i]
		ledgerAddrInfos, err := getLedgerIndexInfo(pClients, index, networks, addr)
		if err != nil {
			return []addressInfo{}, err
		}
		addrInfos = append(addrInfos, ledgerAddrInfos...)
	}
	return addrInfos, nil
}

func getLedgerIndexInfo(
	pClients map[models.Network]platformvm.Client,
	index uint32,
	networks []models.Network,
	addr ids.ShortID,
) ([]addressInfo, error) {
	addrInfos := []addressInfo{}
	for _, network := range networks {
		pChainAddr, err := address.Format("P", key.GetHRP(network.ID), addr[:])
		if err != nil {
			return nil, err
		}
		addrInfo, err := getPChainAddrInfo(
			pClients,
			network,
			pChainAddr,
			"ledger",
			fmt.Sprintf("index %d", index),
		)
		if err != nil {
			return nil, err
		}
		addrInfos = append(addrInfos, addrInfo)
	}
	return addrInfos, nil
}

func getPChainAddrInfo(
	pClients map[models.Network]platformvm.Client,
	network models.Network,
	pChainAddr string,
	kind string,
	name string,
) (addressInfo, error) {
	balance, err := getPChainBalanceStr(pClients[network], pChainAddr)
	if err != nil {
		// just ignore local network errors
		if network.Kind != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    kind,
		name:    name,
		chain:   "P-Chain (Bech32 format)",
		address: pChainAddr,
		balance: balance,
		network: network.Name(),
	}, nil
}

func getXChainAddrInfo(
	xClients map[models.Network]avm.Client,
	network models.Network,
	xChainAddr string,
	kind string,
	name string,
) (addressInfo, error) {
	balance, err := getXChainBalanceStr(xClients[network], xChainAddr)
	if err != nil {
		// just ignore local network errors
		if network.Kind != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    kind,
		name:    name,
		chain:   "X-Chain (Bech32 format)",
		address: xChainAddr,
		balance: balance,
		network: network.Name(),
	}, nil
}

func getEvmBasedChainAddrInfo(
	chainName string,
	cClients map[models.Network]ethclient.Client,
	network models.Network,
	cChainAddr string,
	kind string,
	name string,
) (addressInfo, error) {
	cChainBalance, err := getCChainBalanceStr(cClients[network], cChainAddr)
	if err != nil {
		// just ignore local network errors
		if network.Kind != models.Local {
			return addressInfo{}, err
		}
	}
	return addressInfo{
		kind:    kind,
		name:    name,
		chain:   fmt.Sprintf("%s (Ethereum hex format)", chainName),
		address: cChainAddr,
		balance: cChainBalance,
		network: network.Name(),
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

func getCChainBalanceStr(cClient ethclient.Client, addrStr string) (string, error) {
	addr := common.HexToAddress(addrStr)
	ctx, cancel := utils.GetAPIContext()
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
	balanceStr := ""
	if useNanoAvax {
		balanceStr = fmt.Sprintf("%9d", balance.Uint64())
	} else {
		balanceStr = fmt.Sprintf("%.9f", float64(balance.Uint64())/float64(units.Avax))
	}
	return balanceStr, nil
}

func getPChainBalanceStr(pClient platformvm.Client, addr string) (string, error) {
	pID, err := address.ParseToID(addr)
	if err != nil {
		return "", err
	}
	ctx, cancel := utils.GetAPIContext()
	resp, err := pClient.GetBalance(ctx, []ids.ShortID{pID})
	cancel()
	if err != nil {
		return "", err
	}
	if resp.Balance == 0 {
		return "0", nil
	}
	balanceStr := ""
	if useNanoAvax {
		balanceStr = fmt.Sprintf("%9d", resp.Balance)
	} else {
		balanceStr = fmt.Sprintf("%.9f", float64(resp.Balance)/float64(units.Avax))
	}
	return balanceStr, nil
}

func getXChainBalanceStr(xClient avm.Client, addr string) (string, error) {
	xID, err := address.ParseToID(addr)
	if err != nil {
		return "", err
	}
	ctx, cancel := utils.GetAPIContext()
	asset, err := xClient.GetAssetDescription(ctx, "AVAX")
	if err != nil {
		return "", err
	}
	resp, err := xClient.GetBalance(ctx, xID, asset.AssetID.String(), false)
	cancel()
	if err != nil {
		return "", err
	}
	if resp.Balance == 0 {
		return "0", nil
	}
	balanceStr := ""
	if useNanoAvax {
		balanceStr = fmt.Sprintf("%9d", resp.Balance)
	} else {
		balanceStr = fmt.Sprintf("%.9f", float64(resp.Balance)/float64(units.Avax))
	}
	return balanceStr, nil
}

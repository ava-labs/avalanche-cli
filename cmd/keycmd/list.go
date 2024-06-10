// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
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
	goethereumethclient "github.com/ethereum/go-ethereum/ethclient"
	"github.com/liyue201/erc20-go/erc20"
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
	keysFlag          = "keys"
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
	keys                        []string
	tokenAddresses              []string
	subnetToken                 string
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
	cmd.Flags().StringSliceVar(
		&keys,
		keysFlag,
		[]string{},
		"list addresses for the given keys",
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
	cmd.Flags().StringSliceVar(
		&tokenAddresses,
		"tokens",
		[]string{},
		"provide balance information for the given token contract addresses (Evm only)",
	)
	return cmd
}

type Clients struct {
	x       map[models.Network]avm.Client
	p       map[models.Network]platformvm.Client
	c       map[models.Network]ethclient.Client
	evm     map[models.Network]ethclient.Client
	cGeth   map[models.Network]*goethereumethclient.Client
	evmGeth map[models.Network]*goethereumethclient.Client
}

func getClients(networks []models.Network, pchain bool, cchain bool, xchain bool, subnetName string) (
	*Clients,
	error,
) {
	var err error
	xClients := map[models.Network]avm.Client{}
	pClients := map[models.Network]platformvm.Client{}
	cClients := map[models.Network]ethclient.Client{}
	evmClients := map[models.Network]ethclient.Client{}
	cGethClients := map[models.Network]*goethereumethclient.Client{}
	evmGethClients := map[models.Network]*goethereumethclient.Client{}
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
				return nil, err
			}
			if len(tokenAddresses) != 0 {
				cGethClients[network], err = goethereumethclient.Dial(network.CChainEndpoint())
				if err != nil {
					return nil, err
				}
			}
		}
		if subnetName != "" {
			_, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
			if err != nil {
				return nil, err
			}
			b, _, err := subnetcmd.HasSubnetEVMGenesis(subnetName)
			if err != nil {
				return nil, err
			}
			if b {
				sc, err := app.LoadSidecar(subnetName)
				if err != nil {
					return nil, err
				}
				subnetToken = sc.TokenSymbol
				chainID := sc.Networks[network.Name()].BlockchainID
				if chainID != ids.Empty {
					evmClients[network], err = ethclient.Dial(network.BlockchainEndpoint(chainID.String()))
					if err != nil {
						return nil, err
					}
					if len(tokenAddresses) != 0 {
						evmGethClients[network], err = goethereumethclient.Dial(network.BlockchainEndpoint(chainID.String()))
						if err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	return &Clients{
		p:       pClients,
		x:       xClients,
		c:       cClients,
		evm:     evmClients,
		cGeth:   cGethClients,
		evmGeth: evmGethClients,
	}, nil
}

type addressInfo struct {
	kind    string
	name    string
	chain   string
	token   string
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
			"",
			networkoptions.NetworkFlags{},
			false,
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
	clients, err := getClients(networks, pchain, cchain, xchain, subnetName)
	if err != nil {
		return err
	}
	if queryLedger {
		ledgerIndicesU32 := []uint32{}
		for _, index := range ledgerIndices {
			ledgerIndicesU32 = append(ledgerIndicesU32, uint32(index))
		}
		addrInfos, err = getLedgerIndicesInfo(clients.p, ledgerIndicesU32, networks)
		if err != nil {
			return err
		}
	} else {
		addrInfos, err = getStoredKeysInfo(clients, networks)
		if err != nil {
			return err
		}
	}
	printAddrInfos(addrInfos)
	return nil
}

func getStoredKeysInfo(
	clients *Clients,
	networks []models.Network,
) ([]addressInfo, error) {
	keyNames, err := utils.GetKeyNames(app.GetKeyDir(), true)
	if err != nil {
		return nil, err
	}
	if len(keys) != 0 {
		keyNames = utils.Filter(keyNames, func(keyName string) bool { return utils.Belongs(keys, keyName) })
	}
	addrInfos := []addressInfo{}
	for _, keyName := range keyNames {
		keyAddrInfos, err := getStoredKeyInfo(clients, networks, keyName)
		if err != nil {
			return nil, err
		}
		addrInfos = append(addrInfos, keyAddrInfos...)
	}
	return addrInfos, nil
}

func getStoredKeyInfo(
	clients *Clients,
	networks []models.Network,
	keyName string,
) ([]addressInfo, error) {
	addrInfos := []addressInfo{}
	for _, network := range networks {
		sk, err := app.GetKey(keyName, network, false)
		if err != nil {
			return nil, err
		}
		if _, ok := clients.evm[network]; ok {
			evmAddr := sk.C()
			addrInfo, err := getEvmBasedChainAddrInfo(subnetName, subnetToken, clients.evm, clients.evmGeth, network, evmAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo...)
		}
		if _, ok := clients.c[network]; ok {
			cChainAddr := sk.C()
			addrInfo, err := getEvmBasedChainAddrInfo("C-Chain", "AVAX", clients.c, clients.cGeth, network, cChainAddr, "stored", keyName)
			if err != nil {
				return nil, err
			}
			addrInfos = append(addrInfos, addrInfo...)
		}
		if _, ok := clients.p[network]; ok {
			pChainAddrs := sk.P()
			for _, pChainAddr := range pChainAddrs {
				addrInfo, err := getPChainAddrInfo(clients.p, network, pChainAddr, "stored", keyName)
				if err != nil {
					return nil, err
				}
				addrInfos = append(addrInfos, addrInfo)
			}
		}
		if _, ok := clients.x[network]; ok {
			xChainAddrs := sk.X()
			for _, xChainAddr := range xChainAddrs {
				addrInfo, err := getXChainAddrInfo(clients.x, network, xChainAddr, "stored", keyName)
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
		chain:   "P-Chain",
		token:   "AVAX",
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
		chain:   "X-Chain",
		token:   "AVAX",
		address: xChainAddr,
		balance: balance,
		network: network.Name(),
	}, nil
}

func getEvmBasedChainAddrInfo(
	chainName string,
	chainToken string,
	cClients map[models.Network]ethclient.Client,
	cGethClients map[models.Network]*goethereumethclient.Client,
	network models.Network,
	cChainAddr string,
	kind string,
	name string,
) ([]addressInfo, error) {
	cChainBalance, err := getCChainBalanceStr(cClients[network], cChainAddr)
	if err != nil {
		// just ignore local network errors
		if network.Kind != models.Local {
			return nil, err
		}
	}
	info := addressInfo{
		kind:    kind,
		name:    name,
		chain:   chainName,
		token:   chainToken,
		address: cChainAddr,
		balance: cChainBalance,
		network: network.Name(),
	}
	addressInfos := []addressInfo{info}
	if cGethClients != nil && cGethClients[network] != nil {
		for _, tokenAddress := range tokenAddresses {
			token, err := erc20.NewGGToken(common.HexToAddress(tokenAddress), cGethClients[network])
			if err != nil {
				return addressInfos, err
			}
			tokenName, err := token.Name(nil)
			if err == nil {
				// just ignore contract address access errors as those may depend on network
				balance, err := token.BalanceOf(nil, common.HexToAddress(cChainAddr))
				if err != nil {
					return addressInfos, err
				}
				formattedBalance, err := formatCChainBalance(balance)
				if err != nil {
					return addressInfos, err
				}
				info := addressInfo{
					kind:    kind,
					name:    name,
					chain:   chainName,
					token:   fmt.Sprintf("%s @%s..", tokenName, tokenAddress[:7]),
					address: cChainAddr,
					balance: formattedBalance,
					network: network.Name(),
				}
				addressInfos = append(addressInfos, info)
			}
		}
	}
	return addressInfos, nil
}

func printAddrInfos(addrInfos []addressInfo) {
	header := []string{"Kind", "Name", "Chain", "Address", "Token", "Balance", "Network"}
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
			addrInfo.token,
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
	return formatCChainBalance(balance)
}

func formatCChainBalance(balance *big.Int) (string, error) {
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

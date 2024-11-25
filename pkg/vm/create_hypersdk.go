// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/olekukonko/tablewriter"
)

const (
	InitialBalance uint64 = 10_000_000_000_000
)

// Default preallocation accounts
var ed25519HexKeys = []string{
	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
}

func CreateDefaultHyperSDKGenesis(app *application.Avalanche) ([]byte, error) {
	allocs, err := getPreallocations(app)
	if err != nil {
		return nil, err
	}
	chainID, err := getChainID(app)
	if err != nil {
		return nil, err
	}
	return CreateHyperSDKGenesis(allocs, chainID)
}

func CreateHyperSDKGenesis(allocs []*genesis.CustomAllocation, chainID ids.ID) ([]byte, error) {
	if allocs == nil {
		addrs := make([]codec.Address, len(ed25519HexKeys))
		testKeys := make([]ed25519.PrivateKey, len(ed25519HexKeys))
		for i, keyHex := range ed25519HexKeys {
			bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
			if err != nil {
				return nil, err
			}
			testKeys[i] = ed25519.PrivateKey(bytes)
		}
		for _, key := range testKeys {
			addrs = append(addrs, auth.NewED25519Address(key.PublicKey()))
		}

		customAllocs := make([]*genesis.CustomAllocation, 0, len(addrs))
		for _, account := range addrs {
			customAllocs = append(customAllocs, &genesis.CustomAllocation{
				Address: account,
				Balance: InitialBalance,
			})
		}

		allocs = customAllocs
	}

	genesis := genesis.NewDefaultGenesis(allocs)

	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}

	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = 100

	genesis.Rules.NetworkID = uint32(1)
	genesis.Rules.ChainID = chainID

	return json.Marshal(genesis)
}

func getPreallocations(app *application.Avalanche) ([]*genesis.CustomAllocation, error) {
	defaultOption := "I want to use the default preallocation list in my genesis"
	customOption := "I want to define my own preallocation list"
	option, err := app.Prompt.CaptureList("How would you like to define the preallocation list in your genesis?", []string{defaultOption, customOption})
	if err != nil {
		return nil, err
	}
	switch option {
	case defaultOption:
		return nil, nil
	case customOption:
		balances := make(map[codec.Address]uint64)
		for {
			action, err := app.Prompt.CaptureList("How do you want to proceed?", []string{addAddressAllocationOption, changeAddressAllocationOption, removeAddressAllocationOption, previewAddressAllocationOption, confirmAddressAllocationOption})
			if err != nil {
				return nil, err
			}
			switch action {
			case addAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to add to the initial token allocation")
				if err != nil {
					return nil, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return nil, err
				}
				if _, ok := balances[addr]; ok {
					ux.Logger.PrintToUser("Address already has an allocation entry. Use edit or remove to modify.")
					continue
				}
				balance, err := app.Prompt.CaptureUint64("Enter the initial token balance for this address")
				if err != nil {
					return nil, err
				}
				balances[addr] = balance
			case changeAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to edit the initial token allocation of")
				if err != nil {
					return nil, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return nil, err
				}
				if _, ok := balances[addr]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}
				balance, err := app.Prompt.CaptureUint64("Enter the new initial token balance for this address")
				if err != nil {
					return nil, err
				}
				balances[addr] = balance
			case removeAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to remove from the initial token allocation")
				if err != nil {
					return nil, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return nil, err
				}
				if _, ok := balances[addr]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}
				delete(balances, addr)
			case previewAddressAllocationOption:
				displayHyperSDKDAllocation(balances)
			case confirmAddressAllocationOption:
				customAllocs := make([]*genesis.CustomAllocation, 0, len(balances))
				for account, bal := range balances {
					customAllocs = append(customAllocs, &genesis.CustomAllocation{
						Address: account,
						Balance: bal,
					})
				}
				return customAllocs, nil
			}
		}
	}
	return nil, nil
}

func getChainID(app *application.Avalanche) (ids.ID, error) {
	customChainID := "I want to define my own chain ID"
	defaultChainID := "I don't want to define my own chain ID"
	chainIDOption, err := app.Prompt.CaptureList(
		"How would you like to define the chain ID?",
		[]string{defaultChainID, customChainID},
	)
	if err != nil {
		return ids.Empty, err
	}
	switch chainIDOption {
	case customChainID:
		chainID, err := app.Prompt.CaptureID("Enter the chain ID")
		if err != nil {
			return ids.Empty, err
		}
		return chainID, nil
	case defaultChainID:
		return ids.GenerateTestID(), nil
	}
	return ids.Empty, nil
}

func displayHyperSDKDAllocation(allocs map[codec.Address]uint64) {
	header := []string{"Address", "Balance"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	for addr, balance := range allocs {
		table.Append([]string{addr.String(), fmt.Sprintf("%d", balance)})
	}
	table.Render()
}

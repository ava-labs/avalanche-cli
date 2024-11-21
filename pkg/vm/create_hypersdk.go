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

const InitialBalance uint64 = 10_000_000_000_000

// Default preallocation accounts
var ed25519HexKeys = []string{
	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
}

func CreateHyperSDKGenesis(accounts []codec.Address) ([]byte, error) {
	if len(accounts) == 0 {
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
		accounts = addrs
	}

	customAllocs := make([]*genesis.CustomAllocation, 0, len(accounts))
	for _, account := range accounts {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: account,
			Balance: InitialBalance,
		})
	}

	genesis := genesis.NewDefaultGenesis(customAllocs)

	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}

	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = 100

	genesis.Rules.NetworkID = uint32(1)
	genesis.Rules.ChainID = ids.GenerateTestID()

	return json.Marshal(genesis)
}

func CreateDefaultHyperSDKGenesis(app *application.Avalanche) ([]byte, error) {
	defaultOption := "I want to use the default preallocation list in my genesis"
	customOption := "I want to define my own preallocation list"
	option, err := app.Prompt.CaptureList("How would you like to define the preallocation list in your genesis?", []string{defaultOption, customOption})
	if err != nil {
		return []byte{}, err
	}
	var (
		accounts []codec.Address
	)
	switch option {
	case defaultOption:
		return CreateHyperSDKGenesis(nil)
	case customOption:
		balances := make(map[codec.Address]uint64)
		for {
			action, err := app.Prompt.CaptureList("How do you want to proceed?", []string{addAddressAllocationOption, changeAddressAllocationOption, removeAddressAllocationOption, previewAddressAllocationOption, confirmAddressAllocationOption})
			if err != nil {
				return []byte{}, err
			}
			switch action {
			case addAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to add to the initial token allocation")
				if err != nil {
					return []byte{}, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return []byte{}, err
				}
				if _, ok := balances[addr]; ok {
					ux.Logger.PrintToUser("Address already has an allocation entry. Use edit or remove to modify.")
					continue
				}
				balance, err := app.Prompt.CaptureUint64("Enter the initial token balance for this address")
				if err != nil {
					return []byte{}, err
				}
				balances[addr] = balance
			case changeAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to edit the initial token allocation of")
				if err != nil {
					return []byte{}, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return []byte{}, err
				}
				if _, ok := balances[addr]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}
				balance, err := app.Prompt.CaptureUint64("Enter the new initial token balance for this address")
				if err != nil {
					return []byte{}, err
				}
				balances[addr] = balance
			case removeAddressAllocationOption:
				addrStr, err := app.Prompt.CaptureString("Enter checksummed address to remove from the initial token allocation")
				if err != nil {
					return []byte{}, err
				}
				addr, err := codec.StringToAddress(addrStr)
				if err != nil {
					return []byte{}, err
				}
				if _, ok := balances[addr]; !ok {
					ux.Logger.PrintToUser("Address not found in the allocation list")
					continue
				}
				delete(balances, addr)
			case previewAddressAllocationOption:
				displayHyperSDKDAllocation(balances)
			case confirmAddressAllocationOption:
				for addr := range balances {
					accounts = append(accounts, addr)
				}
				return CreateHyperSDKGenesis(accounts)
			}
		}
	}
	return []byte{}, nil
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

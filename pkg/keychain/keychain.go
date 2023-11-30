// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keychain

import (
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
)

const (
	numLedgerIndicesToSearch           = 1000
	numLedgerIndicesToSearchForBalance = 100
)

var (
	ErrMutuallyExlusiveKeySource = errors.New("key source flags --key, --ewoq, --ledger/--ledger-addrs are mutually exclusive")
	ErrStoredKeyOrEwoqOnMainnet  = errors.New("key sources --key, --ewoq are not available for mainnet operations")
	ErrNonEwoqKeyOnDevnet        = errors.New("key source --ewoq is the only one available for devnet operations")
	ErrEwoqKeyOnFuji             = errors.New("key source --ewoq is not available for fuji operations")
)

type Keychain struct {
	Keychain keychain.Keychain
	UsesLedger bool
	LedgerIndices []uint32
}

func NewKeychain(keychain keychain.Keychain, ledgerIndices []uint32) *Keychain {
	usesLedger := len(ledgerIndices) > 0
	return &Keychain{
		Keychain: keychain,
		UsesLedger: usesLedger,
		LedgerIndices: ledgerIndices,
	}
}

func (kc *Keychain) Addresses() set.Set[ids.ShortID] {
	return kc.Keychain.Addresses()
}

func GetKeychainFromCmdLineFlags(
	app *application.Avalanche,
	keychainGoal string,
	network models.Network,
	keyName string,
	useEwoq bool,
	useLedger *bool,
	ledgerAddresses []string,
	requiredFunds uint64,
) (*Keychain, error) {
	// set ledger usage flag if ledger addresses are given
	if len(ledgerAddresses) > 0 {
		*useLedger = true
	}

	// check mutually exclusive flags
	if !flags.EnsureMutuallyExclusive([]bool{*useLedger, useEwoq, keyName != ""}) {
		return nil, ErrMutuallyExlusiveKeySource
	}

	switch {
	case network.Kind == models.Devnet:
		// going to just use ewoq atm
		useEwoq = true
		if keyName != "" || *useLedger {
			return nil, ErrNonEwoqKeyOnDevnet
		}
	case network.Kind == models.Fuji:
		if useEwoq {
			return nil, ErrEwoqKeyOnFuji
		}
		// prompt the user if no key source was provided
		if !*useLedger && keyName == "" {
			var err error
			*useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, keychainGoal, app.GetKeyDir())
			if err != nil {
				return nil, err
			}
		}
	case network.Kind == models.Mainnet:
		// mainnet requires ledger usage
		if keyName != "" || useEwoq {
			return nil, ErrStoredKeyOrEwoqOnMainnet
		}
		*useLedger = true
	}

	// will use default local keychain if simulating public network opeations on local
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		network = models.LocalNetwork
	}

	// get keychain accessor
	return GetKeychain(app, useEwoq, *useLedger, ledgerAddresses, keyName, network, requiredFunds)
}

func GetKeychain(
	app *application.Avalanche,
	useEwoq bool,
	useLedger bool,
	ledgerAddresses []string,
	keyName string,
	network models.Network,
	requiredFunds uint64,
) (*Keychain, error) {
	// get keychain accessor
	if useLedger {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return nil, err
		}
		// ask for addresses here to print user msg for ledger interaction
		// set ledger indices
		var ledgerIndices []uint32
		if requiredFunds > 0 {
			ledgerIndicesAux, err := searchForFundedLedgerIndices(network, ledgerDevice, requiredFunds)
			if err != nil {
				return nil, err
			}
			ledgerIndices = append(ledgerIndices, ledgerIndicesAux...)
		}
		if len(ledgerAddresses) > 0 {
			ledgerIndicesAux, err := getLedgerIndices(ledgerDevice, ledgerAddresses)
			if err != nil {
				return nil, err
			}
			ledgerIndices = append(ledgerIndices, ledgerIndicesAux...)
		}
		ledgerIndicesSet := set.Set[uint32]{}
		ledgerIndicesSet.Add(ledgerIndices...)
		ledgerIndices = ledgerIndicesSet.List()
		if len(ledgerIndices) == 0 {
			ledgerIndices = []uint32{0}
		}
		// get formatted addresses for ux
		addresses, err := ledgerDevice.Addresses(ledgerIndices)
		if err != nil {
			return nil, err
		}
		addrStrs := []string{}
		for _, addr := range addresses {
			addrStr, err := address.Format("P", key.GetHRP(network.ID), addr[:])
			if err != nil {
				return nil, err
			}
			addrStrs = append(addrStrs, addrStr)
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Ledger addresses: "))
		for _, addrStr := range addrStrs {
			ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("  %s", addrStr)))
		}
		kc, err := keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
		if err != nil {
			return nil, err
		}
		return NewKeychain(kc, ledgerIndices), nil
	}
	if useEwoq {
		sf, err := key.LoadEwoq(network.ID)
		if err != nil {
			return nil, err
		}
		kc := sf.KeyChain()
		return NewKeychain(kc, nil), nil
	}
	sf, err := key.LoadSoft(network.ID, app.GetKeyPath(keyName))
	if err != nil {
		return nil, err
	}
	kc := sf.KeyChain()
	return NewKeychain(kc, nil), nil
}

func getLedgerIndices(ledgerDevice keychain.Ledger, addressesStr []string) ([]uint32, error) {
	addresses, err := address.ParseToIDs(addressesStr)
	if err != nil {
		return []uint32{}, fmt.Errorf("failure parsing ledger addresses: %w", err)
	}
	// maps the indices of addresses to their corresponding ledger indices
	indexMap := map[int]uint32{}
	// for all ledger indices to search for, find if the ledger address belongs to the input
	// addresses and, if so, add the index pair to indexMap, breaking the loop if
	// all addresses were found
	for ledgerIndex := uint32(0); ledgerIndex < numLedgerIndicesToSearch; ledgerIndex++ {
		ledgerAddress, err := ledgerDevice.Addresses([]uint32{ledgerIndex})
		if err != nil {
			return []uint32{}, err
		}
		for addressesIndex, addr := range addresses {
			if addr == ledgerAddress[0] {
				indexMap[addressesIndex] = ledgerIndex
			}
		}
		if len(indexMap) == len(addresses) {
			break
		}
	}
	// create ledgerIndices from indexMap
	ledgerIndices := []uint32{}
	for addressesIndex := range addresses {
		ledgerIndex, ok := indexMap[addressesIndex]
		if !ok {
			continue
		}
		ledgerIndices = append(ledgerIndices, ledgerIndex)
	}
	return ledgerIndices, nil
}

// search for a set of indices that pay a given amount
func searchForFundedLedgerIndices(network models.Network, ledgerDevice keychain.Ledger, amount uint64) ([]uint32, error) {
	ux.Logger.PrintToUser("Looking for ledger indices to pay for %.9f AVAX...", float64(amount)/float64(units.Avax))
	pClient := platformvm.NewClient(network.Endpoint)
	totalBalance := uint64(0)
	ledgerIndices := []uint32{}
	for ledgerIndex := uint32(0); ledgerIndex < numLedgerIndicesToSearchForBalance; ledgerIndex++ {
		ledgerAddress, err := ledgerDevice.Addresses([]uint32{ledgerIndex})
		if err != nil {
			return []uint32{}, err
		}
		ctx, cancel := utils.GetAPIContext()
		resp, err := pClient.GetBalance(ctx, ledgerAddress)
		cancel()
		if err != nil {
			return nil, err
		}
		if resp.Balance > 0 {
			ux.Logger.PrintToUser("  Found index %d with %.9f AVAX", ledgerIndex, float64(resp.Balance)/float64(units.Avax))
			totalBalance += uint64(resp.Balance)
			ledgerIndices = append(ledgerIndices, ledgerIndex)
		}
		if totalBalance >= amount {
			break
		}
	}
	if totalBalance < amount {
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Not enough funds in the first %d indices of Ledger"), numLedgerIndicesToSearchForBalance)
		return nil, fmt.Errorf("not enough funds on ledger")
	}
	return ledgerIndices, nil
}

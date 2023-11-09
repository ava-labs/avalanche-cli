// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
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
	"golang.org/x/exp/slices"
)

var (
	ErrMutuallyExlusiveKeySource = errors.New("key source flags --key, --ewoq, --ledger/--ledger-addrs are mutually exclusive")
	ErrStoredKeyOrEwoqOnMainnet  = errors.New("key sources --key, --ewoq are not available for mainnet operations")
	ErrNonEwoqKeyOnDevnet        = errors.New("key source --ewoq is the only one available for devnet operations")
	ErrEwoqKeyOnFuji             = errors.New("key source --ewoq is not available for fuji operations")
)

func fillNetworkDetails(network *models.Network) error {
	if network.Endpoint == "" {
		endpoint, err := app.Prompt.CaptureString(fmt.Sprintf("%s Network Endpoint", network.Kind.String()))
		if err != nil {
			return err
		}
		network.Endpoint = endpoint
	}
	return nil
}

func GetNetworkFromCmdLineFlags(
	useLocal bool,
	useDevnet bool,
	useFuji bool,
	useMainnet bool,
	endpoint string,
	supportedNetworkKinds []models.NetworkKind,
) (models.Network, error) {
	// get network from flags
	network := models.UndefinedNetwork
	switch {
	case useLocal:
		network = models.LocalNetwork
	case useDevnet:
		network = models.DevnetNetwork
	case useFuji:
		network = models.FujiNetwork
	case useMainnet:
		network = models.MainnetNetwork
	}

	if endpoint != "" {
		network.Endpoint = endpoint
	}

	// no flag was set, prompt user
	if network.Kind == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network for the operation",
			utils.Map(supportedNetworkKinds, func(n models.NetworkKind) string { return n.String() }),
		)
		if err != nil {
			return models.UndefinedNetwork, err
		}
		network = models.NetworkFromString(networkStr)
		if err := fillNetworkDetails(&network); err != nil {
			return models.UndefinedNetwork, err
		}
		return network, nil
	}

	// for err messages
	networkFlags := map[models.NetworkKind]string{
		models.Local:   "--local",
		models.Devnet:  "--devnet",
		models.Fuji:    "--fuji/--testnet",
		models.Mainnet: "--mainnet",
	}
	supportedNetworksFlags := strings.Join(utils.Map(supportedNetworkKinds, func(n models.NetworkKind) string { return networkFlags[n] }), ", ")

	// unsupported network
	if !slices.Contains(supportedNetworkKinds, network.Kind) {
		return models.UndefinedNetwork, fmt.Errorf("network flag %s is not supported. use one of %s", networkFlags[network.Kind], supportedNetworksFlags)
	}

	// not mutually exclusive flag selection
	if !flags.EnsureMutuallyExclusive([]bool{useLocal, useDevnet, useFuji, useMainnet}) {
		return models.UndefinedNetwork, fmt.Errorf("network flags %s are mutually exclusive", supportedNetworksFlags)
	}

	if err := fillNetworkDetails(&network); err != nil {
		return models.UndefinedNetwork, err
	}

	return network, nil
}

func GetKeychainFromCmdLineFlags(
	keychainGoal string,
	network models.Network,
	keyName string,
	useEwoq bool,
	useLedger *bool,
	ledgerAddresses []string,
) (keychain.Keychain, error) {
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
	return GetKeychain(useEwoq, *useLedger, ledgerAddresses, keyName, network)
}

func GetKeychain(
	useEwoq bool,
	useLedger bool,
	ledgerAddresses []string,
	keyName string,
	network models.Network,
) (keychain.Keychain, error) {
	// get keychain accessor
	var kc keychain.Keychain
	if useLedger {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return kc, err
		}
		// ask for addresses here to print user msg for ledger interaction
		// set ledger indices
		var ledgerIndices []uint32
		if len(ledgerAddresses) == 0 {
			ledgerIndices = []uint32{0}
		} else {
			ledgerIndices, err = getLedgerIndices(ledgerDevice, ledgerAddresses)
			if err != nil {
				return kc, err
			}
		}
		// get formatted addresses for ux
		addresses, err := ledgerDevice.Addresses(ledgerIndices)
		if err != nil {
			return kc, err
		}
		addrStrs := []string{}
		for _, addr := range addresses {
			addrStr, err := address.Format("P", key.GetHRP(network.ID), addr[:])
			if err != nil {
				return kc, err
			}
			addrStrs = append(addrStrs, addrStr)
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Ledger addresses: "))
		for _, addrStr := range addrStrs {
			ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("  %s", addrStr)))
		}
		return keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
	}
	if useEwoq {
		sf, err := key.LoadEwoq(network.ID)
		if err != nil {
			return kc, err
		}
		return sf.KeyChain(), nil
	}
	sf, err := key.LoadSoft(network.ID, app.GetKeyPath(keyName))
	if err != nil {
		return kc, err
	}
	return sf.KeyChain(), nil
}

func getLedgerIndices(ledgerDevice keychain.Ledger, addressesStr []string) ([]uint32, error) {
	addresses, err := address.ParseToIDs(addressesStr)
	if err != nil {
		return []uint32{}, fmt.Errorf("failure parsing given ledger addresses: %w", err)
	}
	// maps the indices of addresses to their corresponding ledger indices
	indexMap := map[int]uint32{}
	// for all ledger indices to search for, find if the ledger address belongs to the input
	// addresses and, if so, add the index pair to indexMap, breaking the loop if
	// all addresses were found
	for ledgerIndex := uint32(0); ledgerIndex < numLedgerAddressesToSearch; ledgerIndex++ {
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
			return []uint32{}, fmt.Errorf("address %s not found on ledger", addressesStr[addressesIndex])
		}
		ledgerIndices = append(ledgerIndices, ledgerIndex)
	}
	return ledgerIndices, nil
}

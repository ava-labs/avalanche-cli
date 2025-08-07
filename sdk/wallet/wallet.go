// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package wallet

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/sdk/constants"
	"github.com/ava-labs/avalanche-cli/sdk/keychain"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

var ErrNoAccountsInClient = errors.New("there are no accounts defined in the client")

type Config = primary.WalletConfig

type Wallet struct {
	*primary.Wallet
	Keychain *keychain.Keychain
	options  []common.Option
	config   Config
}

func New(
	network *network.Network,
	keychain *keychain.Keychain,
	config Config,
) (Wallet, error) {
	ctx, cancel := utils.GetTimedContext(constants.WalletCreationTimeout)
	defer cancel()
	wallet, err := primary.MakeWallet(
		ctx,
		network.Endpoint,
		keychain.Keychain,
		secp256k1fx.NewKeychain(),
		config,
	)
	return Wallet{
		Wallet:   wallet,
		Keychain: keychain,
		config:   config,
	}, err
}

// SecureWalletIsChangeOwner ensures that a fee paying address (wallet's keychain) will receive
// the change UTXO and not a randomly selected auth key that may not be paying fees
func (w *Wallet) SecureWalletIsChangeOwner() error {
	addrs := w.Addresses()
	if len(addrs) == 0 {
		return ErrNoAccountsInClient
	}
	changeAddr := addrs[0]
	// sets change to go to wallet addr (instead of any other subnet auth key)
	changeOwner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{changeAddr},
	}
	w.options = append(w.options, common.WithChangeOwner(changeOwner))
	w.Wallet = primary.NewWalletWithOptions(w.Wallet, w.options...)
	return nil
}

// SetAuthKeys sets auth keys that will be used when signing txs, besides the wallet's Keychain fee
// paying ones
func (w *Wallet) SetAuthKeys(authKeys []ids.ShortID) {
	addrs := w.Addresses()
	addrsSet := set.Set[ids.ShortID]{}
	addrsSet.Add(addrs...)
	addrsSet.Add(authKeys...)
	w.options = append(w.options, common.WithCustomAddresses(addrsSet))
	w.Wallet = primary.NewWalletWithOptions(w.Wallet, w.options...)
}

func (w *Wallet) SetSubnetAuthMultisig(authKeys []ids.ShortID) error {
	if err := w.SecureWalletIsChangeOwner(); err != nil {
		return err
	}
	w.SetAuthKeys(authKeys)
	return nil
}

func (w *Wallet) Addresses() []ids.ShortID {
	return w.Keychain.Addresses().List()
}

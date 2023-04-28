// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

var ErrNoSubnetAuthKeysInWallet = errors.New("auth wallet does not contain subnet auth keys")

type PublicDeployer struct {
	LocalDeployer
	usingLedger bool
	kc          keychain.Keychain
	network     models.Network
	app         *application.Avalanche
}

func NewPublicDeployer(app *application.Avalanche, usingLedger bool, kc keychain.Keychain, network models.Network) *PublicDeployer {
	return &PublicDeployer{
		LocalDeployer: *NewLocalDeployer(app, "", ""),
		app:           app,
		usingLedger:   usingLedger,
		kc:            kc,
		network:       network,
	}
}

// adds a subnet validator to the given [subnet]
//   - creates an add subnet validator tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) AddValidator(
	subnetAuthKeysStrs []string,
	subnet ids.ID,
	nodeID ids.NodeID,
	weight uint64,
	startTime time.Time,
	duration time.Duration,
) (bool, *txs.Tx, error) {
	wallet, err := d.loadWallet(subnet)
	if err != nil {
		return false, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}
	validator := &txs.SubnetValidator{
		Validator: txs.Validator{
			NodeID: nodeID,
			Start:  uint64(startTime.Unix()),
			End:    uint64(startTime.Add(duration).Unix()),
			Wght:   weight,
		},
		Subnet: subnet,
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign add validator hash on the ledger device *** ")
	}

	tx, err := d.createAddSubnetValidatorTx(subnetAuthKeys, validator, wallet)
	if err != nil {
		return false, nil, err
	}

	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, d.network, subnet)
	if err != nil {
		return false, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		id, err := d.Commit(tx)
		if err != nil {
			return false, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, nil
}

// removes a subnet validator from the given [subnet]
//   - creates an remove subnet validator tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) RemoveValidator(
	subnetAuthKeysStrs []string,
	subnet ids.ID,
	nodeID ids.NodeID,
) (bool, *txs.Tx, error) {
	wallet, err := d.loadWallet(subnet)
	if err != nil {
		return false, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign remove validator hash on the ledger device *** ")
	}

	tx, err := d.createRemoveValidatorTX(subnetAuthKeys, nodeID, subnet, wallet)
	if err != nil {
		return false, nil, err
	}

	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, d.network, subnet)
	if err != nil {
		return false, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		id, err := d.Commit(tx)
		if err != nil {
			return false, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, nil
}

// deploys the given [chain]
// - creates a subnet using the given [controlKeys] and [threshold] as subnet authentication parameters
// - if operation is multisig (len(subnetAuthKeysStrs) > 1):
//   - creates a blockchain tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and one of the subnet auth keys
//   - returns the tx so that it can be later on be signed by the rest of the subnet auth keys
//
// - if operation is not multisig (len(subnetAuthKeysStrs) == 1):
//   - creates and issues a blockchain tx, signing the tx with the wallet as the owner of fee outputs
//     and the only one subnet auth key
//   - returns the blockchain tx id
func (d *PublicDeployer) Deploy(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	threshold uint32,
	chain string,
	genesis []byte,
) (bool, ids.ID, ids.ID, *txs.Tx, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, err
	}
	vmID, err := utils.VMID(chain)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}

	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, err
	}
	ux.Logger.PrintToUser("Subnet has been created with ID: %s", subnetID.String())

	ux.Logger.PrintToUser("Now creating blockchain...")
	blockchainTx, err := d.createBlockchainTx(subnetAuthKeys, chain, vmID, subnetID, genesis, wallet)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, err
	}

	time.Sleep(2)
	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(blockchainTx, d.network, subnetID)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	var blockchainID ids.ID
	if isFullySigned {
		blockchainID, err = d.Commit(blockchainTx)
		if err != nil {
			return false, ids.Empty, ids.Empty, nil, err
		}
	}

	return isFullySigned, subnetID, blockchainID, blockchainTx, nil
}

func (d *PublicDeployer) Commit(
	tx *txs.Tx,
) (ids.ID, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, err
	}
	return wallet.P().IssueTx(tx)
}

func (d *PublicDeployer) Sign(
	tx *txs.Tx,
	subnetAuthKeysStrs []string,
	subnet ids.ID,
) error {
	wallet, err := d.loadWallet(subnet)
	if err != nil {
		return err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}
	if ok := d.checkWalletHasSubnetAuthAddresses(subnetAuthKeys); !ok {
		return ErrNoSubnetAuthKeysInWallet
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign tx hash on the ledger device *** ")
	}
	if err := d.signTx(tx, wallet); err != nil {
		return err
	}
	return nil
}

func (d *PublicDeployer) loadWallet(preloadTxs ...ids.ID) (primary.Wallet, error) {
	ctx := context.Background()

	var api string
	switch d.network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	case models.Local:
		// used for E2E testing of public related paths
		api = constants.LocalAPIEndpoint
	default:
		return nil, fmt.Errorf("unsupported public network")
	}

	wallet, err := primary.NewWalletWithTxs(ctx, api, d.kc, preloadTxs...)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (d *PublicDeployer) createAndIssueBlockchainTx(
	chainName string,
	vmID,
	subnetID ids.ID,
	genesis []byte,
	wallet primary.Wallet,
) (ids.ID, error) {
	fxIDs := make([]ids.ID, 0)
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign CreateChain transaction on the ledger device *** ")
	}
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName)
}

func (d *PublicDeployer) getMultisigTxOptions(subnetAuthKeys []ids.ShortID) []common.Option {
	options := []common.Option{}
	walletAddr := d.kc.Addresses().List()[0]
	// addrs to use for signing
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(walletAddr)
	customAddrsSet.Add(subnetAuthKeys...)
	options = append(options, common.WithCustomAddresses(customAddrsSet))
	// set change to go to wallet addr (instead of any other subnet auth key)
	changeOwner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{walletAddr},
	}
	options = append(options, common.WithChangeOwner(changeOwner))
	return options
}

func (d *PublicDeployer) createBlockchainTx(
	subnetAuthKeys []ids.ShortID,
	chainName string,
	vmID,
	subnetID ids.ID,
	genesis []byte,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	fxIDs := make([]ids.ID, 0)
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign blockchain creation hash on the ledger device *** ")
	}
	options := d.getMultisigTxOptions(subnetAuthKeys)
	// create tx
	unsignedTx, err := wallet.P().Builder().NewCreateChainTx(
		subnetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		options...,
	)
	if err != nil {
		return nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (d *PublicDeployer) createAddSubnetValidatorTx(
	subnetAuthKeys []ids.ShortID,
	validator *txs.SubnetValidator,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	options := d.getMultisigTxOptions(subnetAuthKeys)
	// create tx
	unsignedTx, err := wallet.P().Builder().NewAddSubnetValidatorTx(validator, options...)
	if err != nil {
		return nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (d *PublicDeployer) createRemoveValidatorTX(
	subnetAuthKeys []ids.ShortID,
	nodeID ids.NodeID,
	subnetID ids.ID,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	options := d.getMultisigTxOptions(subnetAuthKeys)
	// create tx
	unsignedTx, err := wallet.P().Builder().NewRemoveSubnetValidatorTx(nodeID, subnetID, options...)
	if err != nil {
		return nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (*PublicDeployer) signTx(
	tx *txs.Tx,
	wallet primary.Wallet,
) error {
	if err := wallet.P().Signer().Sign(context.Background(), tx); err != nil {
		return err
	}
	return nil
}

func (d *PublicDeployer) createSubnetTx(controlKeys []string, threshold uint32, wallet primary.Wallet) (ids.ID, error) {
	addrs, err := address.ParseToIDs(controlKeys)
	if err != nil {
		return ids.Empty, fmt.Errorf("failure parsing control keys: %w", err)
	}
	owners := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: threshold,
		Locktime:  0,
	}
	opts := []common.Option{}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign CreateSubnet transaction on the ledger device *** ")
	}
	return wallet.P().IssueCreateSubnetTx(owners, opts...)
}

func (d *PublicDeployer) getSubnetAuthAddressesInWallet(subnetAuth []ids.ShortID) []ids.ShortID {
	walletAddrs := d.kc.Addresses().List()
	subnetAuthInWallet := []ids.ShortID{}
	for _, walletAddr := range walletAddrs {
		for _, addr := range subnetAuth {
			if addr == walletAddr {
				subnetAuthInWallet = append(subnetAuthInWallet, addr)
			}
		}
	}
	return subnetAuthInWallet
}

// check that the wallet at least contain one subnet auth address
func (d *PublicDeployer) checkWalletHasSubnetAuthAddresses(subnetAuth []ids.ShortID) bool {
	addrs := d.getSubnetAuthAddressesInWallet(subnetAuth)
	return len(addrs) != 0
}

func IsSubnetValidator(subnetID ids.ID, nodeID ids.NodeID, network models.Network) (bool, error) {
	var apiURL string
	switch network {
	case models.Mainnet:
		apiURL = constants.MainnetAPIEndpoint
	case models.Fuji:
		apiURL = constants.FujiAPIEndpoint
	default:
		return false, fmt.Errorf("invalid network: %s", network)
	}
	pClient := platformvm.NewClient(apiURL)
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, []ids.NodeID{nodeID})
	if err != nil {
		return false, fmt.Errorf("failed to get current validators")
	}

	return !(len(vals) == 0), nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

var ErrNoSubnetAuthKeysInWallet = errors.New("wallet does not contain subnet auth keys")

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
// - verifies that the wallet is one of the subnet auth keys (so as to sign the AddSubnetValidator tx)
// - if operation is multisig (len(subnetAuthKeysStrs) > 1):
//   - creates an add subnet validator tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and one of the subnet auth keys
//   - returns the tx so that it can be later on be signed by the rest of the subnet auth keys
//
// - if operation is not multisig (len(subnetAuthKeysStrs) == 1):
//   - creates and issues an add validator tx, signing the tx with the wallet as the owner of fee outputs
//     and the only one subnet auth key
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
	if ok := d.checkWalletHasSubnetAuthAddresses(subnetAuthKeys); !ok {
		return false, nil, ErrNoSubnetAuthKeysInWallet
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

	if len(subnetAuthKeys) == 1 {
		id, err := wallet.P().IssueAddSubnetValidatorTx(validator)
		if err != nil {
			return false, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, nil, nil
	}

	// not fully signed
	tx, err := d.createAddSubnetValidatorTx(subnetAuthKeys, validator, wallet)
	if err != nil {
		return false, nil, err
	}
	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, nil
}

// deploys the given [chain]
// - verifies that the wallet is one of the subnet auth keys (so as to sign the CreateBlockchain tx)
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

	if ok := d.checkWalletHasSubnetAuthAddresses(subnetAuthKeys); !ok {
		return false, ids.Empty, ids.Empty, nil, ErrNoSubnetAuthKeysInWallet
	}

	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return false, ids.Empty, ids.Empty, nil, err
	}
	ux.Logger.PrintToUser("Subnet has been created with ID: %s. Now creating blockchain...", subnetID.String())

	var (
		blockchainID  ids.ID
		blockchainTx  *txs.Tx
		isFullySigned bool
	)

	if len(subnetAuthKeys) == 1 {
		isFullySigned = true
		blockchainID, err = d.createAndIssueBlockchainTx(chain, vmID, subnetID, genesis, wallet)
		if err != nil {
			return false, ids.Empty, ids.Empty, nil, err
		}
	} else {
		blockchainTx, err = d.createBlockchainTx(subnetAuthKeys, chain, vmID, subnetID, genesis, wallet)
		if err != nil {
			return false, ids.Empty, ids.Empty, nil, err
		}
	}

	return isFullySigned, subnetID, blockchainID, blockchainTx, nil
}

func getAssetID(wallet primary.Wallet) (ids.ID, error) {
	xWallet := wallet.X()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	subnetAssetID, err := xWallet.IssueCreateAssetTx(
		"TEST",
		"TEST",
		9,
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          720 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		return ids.Empty, err
	}
	return subnetAssetID, nil
}

func exportToPChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners, subnetAssetID ids.ID) error {
	xWallet := wallet.X()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)

	_, err := xWallet.IssueExportTx(
		ids.Empty,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: subnetAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          720 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func importFromXChain(wallet primary.Wallet, owner *secp256k1fx.OutputOwners) error {
	xWallet := wallet.X()
	pWallet := wallet.P()
	xChainID := xWallet.BlockchainID()
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWalletCreationTimeout)
	_, err := pWallet.IssueImportTx(
		xChainID,
		owner,
		common.WithContext(ctx),
	)
	defer cancel()
	return err
}

func (d *PublicDeployer) IssueTransformSubnetTx(
	elasticSubnetConfig models.ElasticSubnetConfig,
	subnetID ids.ID,
) (ids.ID, ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, ids.Empty, err
	}
	fmt.Println("getting asset id \n")
	subnetAssetID, err := getAssetID(wallet)
	if err != nil {
		fmt.Println("error obtaining asset id \n")
		return ids.Empty, ids.Empty, err
	}
	fmt.Println("obtained asset id %s \n", subnetAssetID)
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}
	err = exportToPChain(wallet, owner, subnetAssetID)
	if err != nil {
		fmt.Println("error exportToPChain %s \n", err)
		return ids.Empty, ids.Empty, err
	}
	err = importFromXChain(wallet, owner)
	if err != nil {
		fmt.Println("error importFromXChain %s \n", err)
		return ids.Empty, ids.Empty, err
	}
	fmt.Println("we have passed everything here \n")
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultConfirmTxTimeout)
	transformSubnetTxID, err := wallet.P().IssueTransformSubnetTx(elasticSubnetConfig.SubnetID, subnetAssetID,
		elasticSubnetConfig.InitialSupply, elasticSubnetConfig.MaxSupply, elasticSubnetConfig.MinConsumptionRate,
		elasticSubnetConfig.MaxConsumptionRate, elasticSubnetConfig.MinValidatorStake, elasticSubnetConfig.MaxValidatorStake,
		elasticSubnetConfig.MinStakeDuration, elasticSubnetConfig.MaxStakeDuration, elasticSubnetConfig.MinDelegationFee,
		elasticSubnetConfig.MinDelegatorStake, elasticSubnetConfig.MaxValidatorWeightFactor, elasticSubnetConfig.UptimeRequirement,
		common.WithContext(ctx),
	)
	defer cancel()
	if err != nil {
		fmt.Println("error IssueTransformSubnetTx %s \n", err)
		return ids.Empty, ids.Empty, err
	}
	return transformSubnetTxID, subnetAssetID, err
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
		ux.Logger.PrintToUser("*** Please sign blockchain creation hash on the ledger device *** ")
	}
	return wallet.P().IssueCreateChainTx(subnetID, genesis, vmID, fxIDs, chainName)
}

func (d *PublicDeployer) getMultisigTxOptions(subnetAuthKeys []ids.ShortID) []common.Option {
	options := []common.Option{}
	// addrs to use for signing
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(subnetAuthKeys...)
	options = append(options, common.WithCustomAddresses(customAddrsSet))
	// set change to go to wallet addr (instead of any other subnet auth key)
	walletAddresses := d.getSubnetAuthAddressesInWallet(subnetAuthKeys)
	walletAddr := walletAddresses[0]
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
		ux.Logger.PrintToUser("*** Please sign subnet creation hash on the ledger device *** ")
	}
	return wallet.P().IssueCreateSubnetTx(owners, opts...)
}

// get wallet addresses that are also subnet auth addresses
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

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/vms/platformvm/signer"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"

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

// adds a subnet validator to the given [subnetID]
//   - creates an add subnet validator tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) AddValidator(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	nodeID ids.NodeID,
	weight uint64,
	startTime time.Time,
	duration time.Duration,
) (bool, *txs.Tx, []string, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return false, nil, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}
	validator := &txs.SubnetValidator{
		Validator: txs.Validator{
			NodeID: nodeID,
			Start:  uint64(startTime.Unix()),
			End:    uint64(startTime.Add(duration).Unix()),
			Wght:   weight,
		},
		Subnet: subnetID,
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign SubnetValidator transaction on the ledger device *** ")
	}

	tx, err := d.createAddSubnetValidatorTx(subnetAuthKeys, validator, wallet)
	if err != nil {
		return false, nil, nil, err
	}

	_, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return false, nil, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		id, err := d.Commit(tx)
		if err != nil {
			return false, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, nil, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, remainingSubnetAuthKeys, nil
}

// AddValidatorPrimaryNetwork adds node as Primary Network Validator
func (d *PublicDeployer) AddValidatorPrimaryNetwork(
	nodeID ids.NodeID,
	weight uint64,
	startTime time.Time,
	duration time.Duration,
	recipientAddr ids.ShortID,
	shares uint32,
) error {
	wallet, err := d.loadWallet()
	if err != nil {
		return err
	}
	validator := &txs.Validator{
		NodeID: nodeID,
		Start:  uint64(startTime.Unix()),
		End:    uint64(startTime.Add(duration).Unix()),
		Wght:   weight,
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign AddValidator transaction on the ledger device *** ")
	}
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	tx, err := wallet.P().IssueAddValidatorTx(validator, owner, shares)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", tx.ID().String())
	return nil
}

func (d *PublicDeployer) CreateAssetTx(
	subnetID ids.ID,
	tokenName string,
	tokenSymbol string,
	denomination byte,
	initialState map[uint32][]verify.State,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign Create Asset Transaction hash on the ledger device *** ")
	}

	tx, err := wallet.X().IssueCreateAssetTx(tokenName, tokenSymbol, denomination, initialState)
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Create Asset Transaction successful, transaction ID: %s", tx.ID())
	ux.Logger.PrintToUser("Now exporting asset to P-Chain ...")
	return tx.ID(), err
}

func (d *PublicDeployer) ExportToPChainTx(
	subnetID ids.ID,
	subnetAssetID ids.ID,
	owner *secp256k1fx.OutputOwners,
	assetAmount uint64,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign X -> P Chain Export Transaction hash on the ledger device *** ")
	}

	tx, err := wallet.X().IssueExportTx(ids.Empty,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: subnetAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          assetAmount,
					OutputOwners: *owner,
				},
			},
		})
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Export to P-Chain Transaction successful, transaction ID: %s", tx.ID())
	ux.Logger.PrintToUser("Now importing asset from X-Chain ...")
	return tx.ID(), nil
}

func (d *PublicDeployer) ImportFromXChain(
	subnetID ids.ID,
	owner *secp256k1fx.OutputOwners,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign X -> P Chain Import Transaction hash on the ledger device *** ")
	}
	xWallet := wallet.X()
	xChainID := xWallet.BlockchainID()

	tx, err := wallet.P().IssueImportTx(xChainID, owner)
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Import from X Chain Transaction successful, transaction ID: %s", tx.ID())
	ux.Logger.PrintToUser("Now transforming subnet into elastic subnet ...")
	return tx.ID(), err
}

func (d *PublicDeployer) TransformSubnetTx(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	elasticSubnetConfig models.ElasticSubnetConfig,
	subnetID ids.ID,
	subnetAssetID ids.ID,
) (bool, ids.ID, *txs.Tx, []string, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign Transform Subnet hash on the ledger device *** ")
	}

	tx, err := d.createTransformSubnetTX(subnetAuthKeys, elasticSubnetConfig, wallet, subnetAssetID)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}
	_, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		txID, err := d.Commit(tx)
		if err != nil {
			return false, ids.Empty, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", txID)
		return true, txID, nil, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, ids.Empty, tx, remainingSubnetAuthKeys, nil
}

// removes a subnet validator from the given [subnet]
// - verifies that the wallet is one of the subnet auth keys (so as to sign the AddSubnetValidator tx)
// - if operation is multisig (len(subnetAuthKeysStrs) > 1):
//   - creates a remove subnet validator tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) RemoveValidator(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	nodeID ids.NodeID,
) (bool, *txs.Tx, []string, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return false, nil, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign tx hash on the ledger device *** ")
	}

	tx, err := d.createRemoveValidatorTX(subnetAuthKeys, nodeID, subnetID, wallet)
	if err != nil {
		return false, nil, nil, err
	}

	_, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return false, nil, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		id, err := d.Commit(tx)
		if err != nil {
			return false, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, nil, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) AddPermissionlessValidator(
	subnetID ids.ID,
	subnetAssetID ids.ID,
	nodeID ids.NodeID,
	stakeAmount uint64,
	startTime uint64,
	endTime uint64,
	recipientAddr ids.ShortID,
	delegationFee uint32,
	popBytes []byte,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign Add Permissionless Validator hash on the ledger device *** ")
	}
	if subnetAssetID == ids.Empty {
		subnetAssetID = wallet.P().AVAXAssetID()
	}
	txID, err := d.issueAddPermissionlessValidatorTX(recipientAddr, stakeAmount, subnetID, nodeID, subnetAssetID, startTime, endTime, wallet, delegationFee, popBytes)
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", txID)
	return txID, nil
}

func (d *PublicDeployer) AddPermissionlessDelegator(
	subnetID ids.ID,
	subnetAssetID ids.ID,
	nodeID ids.NodeID,
	stakeAmount uint64,
	startTime uint64,
	endTime uint64,
	recipientAddr ids.ShortID,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}
	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign Add Permissionless Validator hash on the ledger device *** ")
	}
	txID, err := d.issueAddPermissionlessDelegatorTX(recipientAddr, stakeAmount, subnetID, nodeID, subnetAssetID, startTime, endTime, wallet)
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", txID)
	return txID, nil
}

// - creates a subnet for [chain] using the given [controlKeys] and [threshold] as subnet authentication parameters
func (d *PublicDeployer) DeploySubnet(
	controlKeys []string,
	threshold uint32,
) (ids.ID, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, err
	}
	subnetID, err := d.createSubnetTx(controlKeys, threshold, wallet)
	if err != nil {
		return ids.Empty, err
	}
	ux.Logger.PrintToUser("Subnet has been created with ID: %s", subnetID.String())
	time.Sleep(2 * time.Second)
	return subnetID, nil
}

// creates a blockchain for the given [subnetID]
//   - creates a create blockchain tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) DeployBlockchain(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	chain string,
	genesis []byte,
) (bool, ids.ID, *txs.Tx, []string, error) {
	ux.Logger.PrintToUser("Now creating blockchain...")

	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}

	vmID, err := utils.VMID(chain)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}

	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	if d.usingLedger {
		ux.Logger.PrintToUser("*** Please sign CreateChain transaction on the ledger device *** ")
	}

	tx, err := d.createBlockchainTx(subnetAuthKeys, chain, vmID, subnetID, genesis, wallet)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}

	_, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	id := ids.Empty
	if isFullySigned {
		id, err = d.Commit(tx)
		if err != nil {
			return false, ids.Empty, nil, nil, err
		}
	}

	return isFullySigned, id, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) Commit(
	tx *txs.Tx,
) (ids.ID, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, err
	}
	err = wallet.P().IssueTx(tx)
	return tx.ID(), err
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
		txName := txutils.GetLedgerDisplayName(tx)
		if len(txName) == 0 {
			ux.Logger.PrintToUser("*** Please sign tx hash on the ledger device *** ")
		} else {
			ux.Logger.PrintToUser("*** Please sign %s transaction on the ledger device *** ", txName)
		}
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
	// filter out ids.Empty txs
	filteredTxs := []ids.ID{}
	for i := range preloadTxs {
		if preloadTxs[i] != ids.Empty {
			filteredTxs = append(filteredTxs, preloadTxs[i])
		}
	}
	wallet, err := primary.NewWalletWithTxs(ctx, api, d.kc, filteredTxs...)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (d *PublicDeployer) getMultisigTxOptions(subnetAuthKeys []ids.ShortID) []common.Option {
	options := []common.Option{}
	walletAddr := d.kc.Addresses().List()[0]
	// addrs to use for signing
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(walletAddr)
	if len(subnetAuthKeys) > 0 {
		customAddrsSet.Add(subnetAuthKeys...)
	}
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

func (d *PublicDeployer) createTransformSubnetTX(
	subnetAuthKeys []ids.ShortID,
	elasticSubnetConfig models.ElasticSubnetConfig,
	wallet primary.Wallet,
	assetID ids.ID,
) (*txs.Tx, error) {
	options := d.getMultisigTxOptions(subnetAuthKeys)
	// create tx
	unsignedTx, err := wallet.P().Builder().NewTransformSubnetTx(elasticSubnetConfig.SubnetID, assetID,
		elasticSubnetConfig.InitialSupply, elasticSubnetConfig.MaxSupply, elasticSubnetConfig.MinConsumptionRate,
		elasticSubnetConfig.MaxConsumptionRate, elasticSubnetConfig.MinValidatorStake, elasticSubnetConfig.MaxValidatorStake,
		elasticSubnetConfig.MinStakeDuration, elasticSubnetConfig.MaxStakeDuration, elasticSubnetConfig.MinDelegationFee,
		elasticSubnetConfig.MinDelegatorStake, elasticSubnetConfig.MaxValidatorWeightFactor, elasticSubnetConfig.UptimeRequirement, options...)
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

// issueAddPermissionlessValidatorTX calls addPermissionlessValidatorTx API on P-Chain
// if subnetID is empty, node nodeID is going to be added as a validator on Primary Network
func (d *PublicDeployer) issueAddPermissionlessValidatorTX(
	recipientAddr ids.ShortID,
	stakeAmount uint64,
	subnetID ids.ID,
	nodeID ids.NodeID,
	assetID ids.ID,
	startTime uint64,
	endTime uint64,
	wallet primary.Wallet,
	delegationFee uint32,
	popBytes []byte,
) (ids.ID, error) {
	options := d.getMultisigTxOptions([]ids.ShortID{})
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	var proofOfPossession signer.Signer
	if subnetID == ids.Empty {
		pop := &signer.ProofOfPossession{}
		err := pop.UnmarshalJSON(popBytes)
		if err != nil {
			return ids.Empty, err
		}
		proofOfPossession = pop
	} else {
		proofOfPossession = &signer.Empty{}
	}
	tx, err := wallet.P().IssueAddPermissionlessValidatorTx(&txs.SubnetValidator{
		Validator: txs.Validator{
			NodeID: nodeID,
			Start:  startTime,
			End:    endTime,
			Wght:   stakeAmount,
		},
		Subnet: subnetID,
	},
		proofOfPossession,
		assetID,
		owner,
		owner,
		delegationFee,
		options...)
	if err != nil {
		return ids.Empty, err
	}
	return tx.ID(), nil
}

func (d *PublicDeployer) issueAddPermissionlessDelegatorTX(
	recipientAddr ids.ShortID,
	stakeAmount uint64,
	subnetID ids.ID,
	nodeID ids.NodeID,
	assetID ids.ID,
	startTime uint64,
	endTime uint64,
	wallet primary.Wallet,
) (ids.ID, error) {
	options := d.getMultisigTxOptions([]ids.ShortID{})
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			recipientAddr,
		},
	}
	tx, err := wallet.P().IssueAddPermissionlessDelegatorTx(
		&txs.SubnetValidator{
			Validator: txs.Validator{
				NodeID: nodeID,
				Start:  startTime,
				End:    endTime,
				Wght:   stakeAmount,
			},
			Subnet: subnetID,
		},
		assetID,
		owner,
		options...,
	)
	if err != nil {
		return ids.Empty, err
	}
	return tx.ID(), nil
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
	tx, err := wallet.P().IssueCreateSubnetTx(owners, opts...)
	if err != nil {
		return ids.Empty, err
	}
	return tx.ID(), nil
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

func GetPublicSubnetValidators(subnetID ids.ID, network models.Network) ([]platformvm.ClientPermissionlessValidator, error) {
	var apiURL string
	switch network {
	case models.Mainnet:
		apiURL = constants.MainnetAPIEndpoint
	case models.Fuji:
		apiURL = constants.FujiAPIEndpoint
	default:
		return nil, fmt.Errorf("invalid network: %s", network)
	}
	pClient := platformvm.NewClient(apiURL)
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, []ids.NodeID{})
	if err != nil {
		return nil, fmt.Errorf("failed to get current validators")
	}

	return vals, nil
}

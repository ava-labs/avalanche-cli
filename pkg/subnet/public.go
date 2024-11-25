// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"errors"
	"fmt"
	"time"

	avagofee "github.com/ava-labs/avalanchego/vms/platformvm/txs/fee"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	goethereumcommon "github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	avmtxs "github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

const showFees = true

var ErrNoSubnetAuthKeysInWallet = errors.New("auth wallet does not contain subnet auth keys")

type PublicDeployer struct {
	LocalDeployer
	kc      *keychain.Keychain
	network models.Network
	app     *application.Avalanche
	wallet  primary.Wallet
}

func NewPublicDeployer(app *application.Avalanche, kc *keychain.Keychain, network models.Network) *PublicDeployer {
	return &PublicDeployer{
		LocalDeployer: *NewLocalDeployer(app, "", "", "", false),
		app:           app,
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
func (d *PublicDeployer) AddValidatorNonSOV(
	waitForTxAcceptance bool,
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	nodeID ids.NodeID,
	weight uint64,
	startTime time.Time,
	duration time.Duration,
) (bool, *txs.Tx, []string, error) {
	wallet, err := d.loadCacheWallet(subnetID)
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
	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "SubnetValidator transaction")

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
		id, err := d.Commit(tx, waitForTxAcceptance)
		if err != nil {
			return false, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, tx, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) SetL1ValidatorWeight(
	message *warp.Message,
) (ids.ID, *txs.Tx, error) {
	wallet, err := d.loadCacheWallet()
	if err != nil {
		return ids.Empty, nil, err
	}
	tx, err := d.createSetSubnetValidatorWeightTx(
		message,
		wallet,
	)
	if err != nil {
		return ids.Empty, nil, err
	}
	id, err := d.Commit(tx, true)
	return id, tx, err
}

func (*PublicDeployer) createSetSubnetValidatorWeightTx(
	message *warp.Message,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	unsignedTx, err := wallet.P().Builder().NewSetL1ValidatorWeightTx(
		message.Bytes(),
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	if unsignedTx != nil {
		if err := printFee("SetL1ValidatorWeightTx", wallet, unsignedTx); err != nil {
			return nil, err
		}
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
	}
	return &tx, nil
}

func (d *PublicDeployer) RegisterL1Validator(
	balance uint64,
	pop signer.ProofOfPossession,
	message *warp.Message,
) (ids.ID, *txs.Tx, error) {
	wallet, err := d.loadCacheWallet()
	if err != nil {
		return ids.Empty, nil, err
	}
	tx, err := d.createRegisterSubnetValidatorTx(
		balance,
		pop,
		message,
		wallet,
	)
	if err != nil {
		return ids.Empty, nil, err
	}
	id, err := d.Commit(tx, true)
	return id, tx, err
}

func (*PublicDeployer) createRegisterSubnetValidatorTx(
	balance uint64,
	pop signer.ProofOfPossession,
	message *warp.Message,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	unsignedTx, err := wallet.P().Builder().NewRegisterL1ValidatorTx(
		balance,
		pop.ProofOfPossession,
		message.Bytes(),
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	if unsignedTx != nil {
		if err := printFee("RegisterL1ValidatorTx", wallet, unsignedTx); err != nil {
			return nil, err
		}
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
	}
	return &tx, nil
}

// change subnet owner for [subnetID]
//   - creates a transfer subnet ownership tx
//   - sets the change output owner to be a wallet address (if not, it may go to any other subnet auth address)
//   - signs the tx with the wallet as the owner of fee outputs and a possible subnet auth key
//   - if partially signed, returns the tx so that it can later on be signed by the rest of the subnet auth keys
//   - if fully signed, issues it
func (d *PublicDeployer) TransferSubnetOwnership(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	newControlKeys []string,
	newThreshold uint32,
) (bool, *txs.Tx, []string, error) {
	wallet, err := d.loadCacheWallet(subnetID)
	if err != nil {
		return false, nil, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}
	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "TransferSubnetOwnership transaction")

	tx, err := d.createTransferSubnetOwnershipTx(
		subnetAuthKeys,
		subnetID,
		newControlKeys,
		newThreshold,
		wallet,
	)
	if err != nil {
		return false, nil, nil, err
	}

	_, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return false, nil, nil, err
	}
	isFullySigned := len(remainingSubnetAuthKeys) == 0

	if isFullySigned {
		id, err := d.Commit(tx, true)
		if err != nil {
			return false, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, tx, nil, nil
	}

	ux.Logger.PrintToUser("Partial tx created")
	return false, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) CreateAssetTx(
	tokenName string,
	tokenSymbol string,
	denomination byte,
	initialState map[uint32][]verify.State,
) (ids.ID, error) {
	wallet, err := d.loadWallet()
	if err != nil {
		return ids.Empty, err
	}

	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "Create Asset transaction hash")

	unsignedTx, err := wallet.X().Builder().NewCreateAssetTx(
		tokenName,
		tokenSymbol,
		denomination,
		initialState,
	)
	if err != nil {
		return ids.Empty, fmt.Errorf("error building tx: %w", err)
	}
	tx := avmtxs.Tx{Unsigned: unsignedTx}
	if err := wallet.X().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, fmt.Errorf("error signing tx: %w", err)
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.X().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return ids.Empty, err
	}

	ux.Logger.PrintToUser("Create Asset Transaction successful, transaction ID: %s", tx.ID())
	ux.Logger.PrintToUser("Now exporting asset to P-Chain ...")
	return tx.ID(), err
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
	wallet, err := d.loadCacheWallet(subnetID)
	if err != nil {
		return false, nil, nil, err
	}
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "tx hash")

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
		id, err := d.Commit(tx, true)
		if err != nil {
			return false, nil, nil, err
		}
		ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", id)
		return true, tx, nil, nil
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
	proofOfPossession *signer.ProofOfPossession,
) (ids.ID, error) {
	wallet, err := d.loadWallet(subnetID)
	if err != nil {
		return ids.Empty, err
	}
	if subnetAssetID == ids.Empty {
		subnetAssetID = wallet.P().Builder().Context().AVAXAssetID
	}
	// popBytes is a marshalled json object containing publicKey and proofOfPossession of the node's BLS info
	txID, err := d.issueAddPermissionlessValidatorTX(recipientAddr, stakeAmount, subnetID, nodeID, subnetAssetID, startTime, endTime, wallet, delegationFee, popBytes, proofOfPossession)
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

	wallet, err := d.loadCacheWallet(subnetID)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}

	vmID, err := anrutils.VMID(chain)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}

	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "CreateChain transaction")

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
		id, err = d.Commit(tx, true)
		if err != nil {
			return false, ids.Empty, nil, nil, err
		}
	}

	return isFullySigned, id, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) ConvertL1(
	controlKeys []string,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
	chainID ids.ID,
	validatorManagerAddress goethereumcommon.Address,
	validators []*txs.ConvertSubnetToL1Validator,
) (bool, ids.ID, *txs.Tx, []string, error) {
	ux.Logger.PrintToUser("Now calling ConvertSubnetToL1Tx...")

	wallet, err := d.loadCacheWallet(subnetID)
	if err != nil {
		return false, ids.Empty, nil, nil, err
	}

	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStrs)
	if err != nil {
		return false, ids.Empty, nil, nil, fmt.Errorf("failure parsing subnet auth keys: %w", err)
	}

	showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "ConvertSubnetToL1Tx")

	tx, err := d.createConvertL1Tx(subnetAuthKeys, subnetID, chainID, validatorManagerAddress.Bytes(), validators, wallet)
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
		id, err = d.Commit(tx, true)
		if err != nil {
			return false, ids.Empty, nil, nil, err
		}
	}

	return isFullySigned, id, tx, remainingSubnetAuthKeys, nil
}

func (d *PublicDeployer) PChainTransfer(
	destination ids.ShortID,
	amount uint64,
) (ids.ID, *txs.Tx, error) {
	wallet, err := d.loadCacheWallet()
	if err != nil {
		return ids.Empty, nil, err
	}
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{destination},
	}
	output := &avax.TransferableOutput{
		Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt:          amount,
			OutputOwners: to,
		},
	}
	outputs := []*avax.TransferableOutput{output}
	unsignedTx, err := wallet.P().Builder().NewBaseTx(
		outputs,
	)
	if err != nil {
		return ids.Empty, nil, err
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, nil, err
	}
	id, err := d.Commit(&tx, true)
	if err != nil {
		return ids.Empty, nil, err
	}
	return id, &tx, nil
}

func (d *PublicDeployer) Commit(
	tx *txs.Tx,
	waitForTxAcceptance bool,
) (ids.ID, error) {
	const (
		repeats             = 3
		sleepBetweenRepeats = 2 * time.Second
	)
	var issueTxErr error
	wallet, err := d.loadCacheWallet()
	if err != nil {
		return ids.Empty, err
	}
	for i := 0; i < repeats; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		options := []common.Option{common.WithContext(ctx)}
		if !waitForTxAcceptance {
			options = append(options, common.WithAssumeDecided())
		}
		issueTxErr = wallet.P().IssueTx(tx, options...)
		if issueTxErr == nil {
			break
		}
		if ctx.Err() != nil {
			issueTxErr = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), issueTxErr)
		} else {
			issueTxErr = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), issueTxErr)
		}
		ux.Logger.RedXToUser("%s", issueTxErr)
		time.Sleep(sleepBetweenRepeats)
	}
	if issueTxErr != nil {
		d.CleanCacheWallet()
	}
	return tx.ID(), issueTxErr
}

func (d *PublicDeployer) Sign(
	tx *txs.Tx,
	subnetAuthKeysStrs []string,
	subnetID ids.ID,
) error {
	wallet, err := d.loadWallet(subnetID)
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
	if d.kc.UsesLedger {
		txName := txutils.GetLedgerDisplayName(tx)
		if len(txName) == 0 {
			showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "tx hash")
		} else {
			showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), fmt.Sprintf("%s transaction", txName))
		}
	}
	if err := d.signTx(tx, wallet); err != nil {
		return err
	}
	return nil
}

func (d *PublicDeployer) loadWallet(subnetIDs ...ids.ID) (primary.Wallet, error) {
	ctx := context.Background()
	// filter out ids.Empty txs
	filteredTxs := utils.Filter(subnetIDs, func(e ids.ID) bool { return e != ids.Empty })
	wallet, err := primary.MakeWallet(
		ctx,
		&primary.WalletConfig{
			URI:          d.network.Endpoint,
			AVAXKeychain: d.kc.Keychain,
			EthKeychain:  secp256k1fx.NewKeychain(),
			SubnetIDs:    filteredTxs,
		},
	)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (d *PublicDeployer) CleanCacheWallet() {
	d.wallet = nil
}

func (d *PublicDeployer) loadCacheWallet(preloadTxs ...ids.ID) (primary.Wallet, error) {
	var err error
	if d.wallet == nil {
		d.wallet, err = d.loadWallet(preloadTxs...)
	}
	return d.wallet, err
}

func (d *PublicDeployer) getMultisigTxOptions(subnetAuthKeys []ids.ShortID) []common.Option {
	options := []common.Option{}
	walletAddrs := d.kc.Addresses().List()
	changeAddr := walletAddrs[0]
	// addrs to use for signing
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(walletAddrs...)
	customAddrsSet.Add(subnetAuthKeys...)
	options = append(options, common.WithCustomAddresses(customAddrsSet))
	// set change to go to wallet addr (instead of any other subnet auth key)
	changeOwner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{changeAddr},
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
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	if unsignedTx != nil {
		if err := printFee("CreateChainTx", wallet, unsignedTx); err != nil {
			return nil, err
		}
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
	}
	return &tx, nil
}

func (d *PublicDeployer) createConvertL1Tx(
	subnetAuthKeys []ids.ShortID,
	subnetID ids.ID,
	chainID ids.ID,
	address []byte,
	validators []*txs.ConvertSubnetToL1Validator,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	options := d.getMultisigTxOptions(subnetAuthKeys)
	unsignedTx, err := wallet.P().Builder().NewConvertSubnetToL1Tx(
		subnetID,
		chainID,
		address,
		validators,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	if unsignedTx != nil {
		if err := printFee("ConvertSubnetToL1Tx", wallet, unsignedTx); err != nil {
			return nil, err
		}
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
	}
	return &tx, nil
}

func (d *PublicDeployer) createTransferSubnetOwnershipTx(
	subnetAuthKeys []ids.ShortID,
	subnetID ids.ID,
	controlKeys []string,
	threshold uint32,
	wallet primary.Wallet,
) (*txs.Tx, error) {
	options := d.getMultisigTxOptions(subnetAuthKeys)
	addrs, err := address.ParseToIDs(controlKeys)
	if err != nil {
		return nil, fmt.Errorf("failure parsing control keys: %w", err)
	}
	owner := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: threshold,
		Locktime:  0,
	}
	// create tx
	unsignedTx, err := wallet.P().Builder().NewTransferSubnetOwnershipTx(
		subnetID,
		owner,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
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
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
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
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	// sign with current wallet
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return nil, fmt.Errorf("error signing tx: %w", err)
	}
	return &tx, nil
}

// issueAddPermissionlessValidatorTX calls addPermissionlessValidatorTx API on P-Chain
// if subnetID is empty, node nodeID is going to be added as a validator on Primary Network
// if popBytes is empty, that means that we are using BLS proof generated from signer.key file
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
	blsProof *signer.ProofOfPossession,
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
		if popBytes != nil {
			pop := &signer.ProofOfPossession{}
			err := pop.UnmarshalJSON(popBytes)
			if err != nil {
				return ids.Empty, err
			}
			proofOfPossession = pop
		} else {
			proofOfPossession = blsProof
		}
	} else {
		proofOfPossession = &signer.Empty{}
	}

	if d.kc.UsesLedger {
		showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "Add Permissionless Validator hash")
	}
	unsignedTx, err := wallet.P().Builder().NewAddPermissionlessValidatorTx(
		&txs.SubnetValidator{
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
		options...,
	)
	if err != nil {
		return ids.Empty, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, fmt.Errorf("error signing tx: %w", err)
	}

	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.P().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return ids.Empty, err
	}

	return tx.ID(), nil
}

func (*PublicDeployer) signTx(
	tx *txs.Tx,
	wallet primary.Wallet,
) error {
	if err := wallet.P().Signer().Sign(context.Background(), tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
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
	if d.kc.UsesLedger {
		showLedgerSignatureMsg(d.kc.UsesLedger, d.kc.HasOnlyOneKey(), "CreateSubnet transaction")
	}
	unsignedTx, err := wallet.P().Builder().NewCreateSubnetTx(
		owners,
	)
	if unsignedTx != nil {
		if err := printFee("CreateSubnetTx", wallet, unsignedTx); err != nil {
			return ids.Empty, err
		}
	}
	if err != nil {
		return ids.Empty, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, fmt.Errorf("error signing tx: %w", err)
	}

	return d.Commit(&tx, true)
}

func printFee(kind string, wallet primary.Wallet, unsignedTx txs.UnsignedTx) error {
	if showFees {
		var pFeeCalculator avagofee.Calculator
		pContext := wallet.P().Builder().Context()
		calcKind := "dynamic"
		if pContext.GasPrice != 0 {
			pFeeCalculator = avagofee.NewDynamicCalculator(pContext.ComplexityWeights, pContext.GasPrice)
		} else {
			pFeeCalculator = avagofee.NewStaticCalculator(pContext.StaticFeeConfig)
			calcKind = "static"
		}
		txFee, err := pFeeCalculator.CalculateFee(unsignedTx)
		if err != nil {
			if !errors.Is(err, avagofee.ErrUnsupportedTx) {
				return err
			}
			ux.Logger.PrintToUser(logging.Yellow.Wrap("unable to get %s fee: not supported by %s calculator"), kind, calcKind)
		} else {
			ux.Logger.PrintToUser(logging.Yellow.Wrap("%s fee: %.9f AVAX"), kind, float64(txFee)/float64(units.Avax))
		}
	}
	return nil
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
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, []ids.NodeID{nodeID})
	if err != nil {
		return false, fmt.Errorf("failed to get current validators")
	}

	return !(len(vals) == 0), nil
}

func GetPublicSubnetValidators(subnetID ids.ID, network models.Network) ([]platformvm.ClientPermissionlessValidator, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	vals, err := pClient.GetCurrentValidators(ctx, subnetID, []ids.NodeID{})
	if err != nil {
		return nil, fmt.Errorf("failed to get current validators")
	}

	return vals, nil
}

func IssueXToPExportTx(
	wallet primary.Wallet,
	usingLedger bool,
	hasOnlyOneKey bool,
	assetID ids.ID,
	amount uint64,
	owner *secp256k1fx.OutputOwners,
) (ids.ID, error) {
	showLedgerSignatureMsg(usingLedger, hasOnlyOneKey, "X -> P Chain Export Transaction")
	unsignedTx, err := wallet.X().Builder().NewExportTx(
		avagoconstants.PlatformChainID,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: assetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          amount,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		return ids.Empty, fmt.Errorf("error building tx: %w", err)
	}
	tx := avmtxs.Tx{Unsigned: unsignedTx}
	if err := wallet.X().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.X().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return tx.ID(), err
	}
	return tx.ID(), nil
}

func IssuePFromXImportTx(
	wallet primary.Wallet,
	usingLedger bool,
	hasOnlyOneKey bool,
	owner *secp256k1fx.OutputOwners,
) (ids.ID, error) {
	showLedgerSignatureMsg(usingLedger, hasOnlyOneKey, "X -> P Chain Import Transaction")
	unsignedTx, err := wallet.P().Builder().NewImportTx(
		wallet.X().Builder().Context().BlockchainID,
		owner,
	)
	if err != nil {
		return ids.Empty, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return ids.Empty, fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.P().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return tx.ID(), err
	}
	return tx.ID(), err
}

func showLedgerSignatureMsg(
	usingLedger bool,
	hasOnlyOneKey bool,
	toSignDesc string,
) {
	multipleTimesMsg := ""
	if !hasOnlyOneKey {
		multipleTimesMsg = logging.LightBlue.Wrap("(you may be asked more than once) ")
	}
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign %s on the ledger device %s***", toSignDesc, multipleTimesMsg)
	}
}

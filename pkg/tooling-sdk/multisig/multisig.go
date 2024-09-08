// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package multisig

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/ava-labs/avalanche-cli/pkg/tooling-sdk/avalancheSDK"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

type TxKind int64

var ErrUndefinedTx = fmt.Errorf("tx is undefined")

const (
	Undefined TxKind = iota
	PChainRemoveSubnetValidatorTx
	PChainAddSubnetValidatorTx
	PChainCreateChainTx
	PChainTransformSubnetTx
	PChainAddPermissionlessValidatorTx
	PChainTransferSubnetOwnershipTx
)

type Multisig struct {
	PChainTx    *txs.Tx
	controlKeys []ids.ShortID
	threshold   uint32
}

func New(pChainTx *txs.Tx) *Multisig {
	ms := Multisig{
		PChainTx: pChainTx,
	}
	return &ms
}

func (ms *Multisig) String() string {
	if ms.PChainTx != nil {
		return ms.PChainTx.ID().String()
	}
	return ""
}

func (ms *Multisig) Undefined() bool {
	return ms.PChainTx == nil
}

func (ms *Multisig) ToBytes() ([]byte, error) {
	if ms.Undefined() {
		return nil, ErrUndefinedTx
	}
	txBytes, err := txs.Codec.Marshal(txs.CodecVersion, ms.PChainTx)
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal signed tx: %w", err)
	}
	return txBytes, nil
}

func (ms *Multisig) ToFile(txPath string) error {
	if ms.Undefined() {
		return ErrUndefinedTx
	}
	txBytes, err := ms.ToBytes()
	if err != nil {
		return err
	}
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode signed tx: %w", err)
	}
	f, err := os.Create(txPath)
	if err != nil {
		return fmt.Errorf("couldn't create file to write tx to: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(txStr)
	if err != nil {
		return fmt.Errorf("couldn't write tx into file: %w", err)
	}
	return nil
}

func (ms *Multisig) FromBytes(txBytes []byte) error {
	var tx txs.Tx
	if _, err := txs.Codec.Unmarshal(txBytes, &tx); err != nil {
		return fmt.Errorf("error unmarshaling signed tx: %w", err)
	}
	if err := tx.Initialize(txs.Codec); err != nil {
		return fmt.Errorf("error initializing signed tx: %w", err)
	}
	ms.PChainTx = &tx
	return nil
}

func (ms *Multisig) FromFile(txPath string) error {
	txEncodedBytes, err := os.ReadFile(txPath)
	if err != nil {
		return err
	}
	txBytes, err := formatting.Decode(formatting.Hex, string(txEncodedBytes))
	if err != nil {
		return fmt.Errorf("couldn't decode signed tx: %w", err)
	}
	return ms.FromBytes(txBytes)
}

func (ms *Multisig) IsReadyToCommit() (bool, error) {
	if ms.Undefined() {
		return false, ErrUndefinedTx
	}
	unsignedTx := ms.PChainTx.Unsigned
	switch unsignedTx.(type) {
	case *txs.CreateSubnetTx:
		return true, nil
	default:
	}
	_, remainingSigners, err := ms.GetRemainingAuthSigners()
	if err != nil {
		return false, err
	}
	return len(remainingSigners) == 0, nil
}

// GetRemainingAuthSigners gets subnet auth addresses that have not signed a given tx
//   - get the string slice of auth signers for the tx (GetAuthSigners)
//   - verifies that all creds in tx.Creds, except the last one, are fully signed
//     (a cred is fully signed if all the signatures in cred.Sigs are non-empty)
//   - computes remaining signers by iterating the last cred in tx.Creds, associated to subnet auth signing
//   - for each sig in cred.Sig: if sig is empty, then add the associated auth signer address (obtained from
//     authSigners by using the index) to the remaining signers list
//
// if the tx is fully signed, returns empty slice
func (ms *Multisig) GetRemainingAuthSigners() ([]ids.ShortID, []ids.ShortID, error) {
	if ms.Undefined() {
		return nil, nil, ErrUndefinedTx
	}
	authSigners, err := ms.GetAuthSigners()
	if err != nil {
		return nil, nil, err
	}
	emptySig := [secp256k1.SignatureLen]byte{}
	numCreds := len(ms.PChainTx.Creds)
	// we should have at least 1 cred for output owners and 1 cred for subnet auth
	if numCreds < 2 {
		return nil, nil, fmt.Errorf("expected tx.Creds of len 2, got %d. doesn't seem to be a multisig tx with subnet auth requirements", numCreds)
	}
	// signatures for output owners should be filled (all creds except last one)
	for credIndex := range ms.PChainTx.Creds[:numCreds-1] {
		cred, ok := ms.PChainTx.Creds[credIndex].(*secp256k1fx.Credential)
		if !ok {
			return nil, nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", ms.PChainTx.Creds[credIndex])
		}
		for i, sig := range cred.Sigs {
			if sig == emptySig {
				return nil, nil, fmt.Errorf("expected funding sig %d of cred %d to be filled", i, credIndex)
			}
		}
	}
	// signatures for subnet auth (last cred)
	cred, ok := ms.PChainTx.Creds[numCreds-1].(*secp256k1fx.Credential)
	if !ok {
		return nil, nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", ms.PChainTx.Creds[1])
	}
	if len(cred.Sigs) != len(authSigners) {
		return nil, nil, fmt.Errorf("expected number of cred's signatures %d to equal number of auth signers %d",
			len(cred.Sigs),
			len(authSigners),
		)
	}
	remainingSigners := []ids.ShortID{}
	for i, sig := range cred.Sigs {
		if sig == emptySig {
			remainingSigners = append(remainingSigners, authSigners[i])
		}
	}
	return authSigners, remainingSigners, nil
}

// GetAuthSigners gets all subnet auth addresses that are required to sign a given tx
//   - get subnet control keys as string slice using P-Chain API (GetOwners)
//   - get subnet auth indices from the tx, field tx.UnsignedTx.SubnetAuth
//   - creates the string slice of required subnet auth addresses by applying
//     the indices to the control keys slice
func (ms *Multisig) GetAuthSigners() ([]ids.ShortID, error) {
	if ms.Undefined() {
		return nil, ErrUndefinedTx
	}
	controlKeys, _, err := ms.GetSubnetOwners()
	if err != nil {
		return nil, err
	}
	unsignedTx := ms.PChainTx.Unsigned
	var subnetAuth verify.Verifiable
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.AddSubnetValidatorTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.CreateChainTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.TransformSubnetTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.TransferSubnetOwnershipTx:
		subnetAuth = unsignedTx.SubnetAuth
	default:
		return nil, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	subnetInput, ok := subnetAuth.(*secp256k1fx.Input)
	if !ok {
		return nil, fmt.Errorf("expected subnetAuth of type *secp256k1fx.Input, got %T", subnetAuth)
	}
	authSigners := []ids.ShortID{}
	for _, sigIndex := range subnetInput.SigIndices {
		if sigIndex >= uint32(len(controlKeys)) {
			return nil, fmt.Errorf("signer index %d exceeds number of control keys", sigIndex)
		}
		authSigners = append(authSigners, controlKeys[sigIndex])
	}
	return authSigners, nil
}

func (*Multisig) GetSpendSigners() ([]ids.ShortID, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (ms *Multisig) GetTxKind() (TxKind, error) {
	if ms.Undefined() {
		return Undefined, ErrUndefinedTx
	}
	unsignedTx := ms.PChainTx.Unsigned
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		return PChainRemoveSubnetValidatorTx, nil
	case *txs.AddSubnetValidatorTx:
		return PChainAddSubnetValidatorTx, nil
	case *txs.CreateChainTx:
		return PChainCreateChainTx, nil
	case *txs.TransformSubnetTx:
		return PChainTransformSubnetTx, nil
	case *txs.AddPermissionlessValidatorTx:
		return PChainAddPermissionlessValidatorTx, nil
	case *txs.TransferSubnetOwnershipTx:
		return PChainTransferSubnetOwnershipTx, nil
	default:
		return Undefined, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
}

// get network id associated to tx
func (ms *Multisig) GetNetworkID() (uint32, error) {
	if ms.Undefined() {
		return 0, ErrUndefinedTx
	}
	unsignedTx := ms.PChainTx.Unsigned
	var networkID uint32
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		networkID = unsignedTx.NetworkID
	case *txs.AddSubnetValidatorTx:
		networkID = unsignedTx.NetworkID
	case *txs.CreateChainTx:
		networkID = unsignedTx.NetworkID
	case *txs.TransformSubnetTx:
		networkID = unsignedTx.NetworkID
	case *txs.AddPermissionlessValidatorTx:
		networkID = unsignedTx.NetworkID
	case *txs.TransferSubnetOwnershipTx:
		networkID = unsignedTx.NetworkID
	default:
		return 0, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	return networkID, nil
}

// get network model associated to tx
func (ms *Multisig) GetNetwork() (avalancheSDK.Network, error) {
	if ms.Undefined() {
		return avalancheSDK.UndefinedNetwork, ErrUndefinedTx
	}
	networkID, err := ms.GetNetworkID()
	if err != nil {
		return avalancheSDK.UndefinedNetwork, err
	}
	network := avalancheSDK.NetworkFromNetworkID(networkID)
	if network.Kind == avalancheSDK.Undefined {
		return avalancheSDK.UndefinedNetwork, fmt.Errorf("undefined network model for tx")
	}
	return network, nil
}

func (ms *Multisig) GetBlockchainID() (ids.ID, error) {
	if ms.Undefined() {
		return ids.Empty, ErrUndefinedTx
	}
	unsignedTx := ms.PChainTx.Unsigned
	var blockchainID ids.ID
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		blockchainID = unsignedTx.BlockchainID
	case *txs.AddSubnetValidatorTx:
		blockchainID = unsignedTx.BlockchainID
	case *txs.CreateChainTx:
		blockchainID = unsignedTx.BlockchainID
	case *txs.TransformSubnetTx:
		blockchainID = unsignedTx.BlockchainID
	case *txs.AddPermissionlessValidatorTx:
		blockchainID = unsignedTx.BlockchainID
	case *txs.TransferSubnetOwnershipTx:
		blockchainID = unsignedTx.BlockchainID
	default:
		return ids.Empty, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	return blockchainID, nil
}

// GetSubnetID gets subnet id associated to tx
func (ms *Multisig) GetSubnetID() (ids.ID, error) {
	if ms.Undefined() {
		return ids.Empty, ErrUndefinedTx
	}
	unsignedTx := ms.PChainTx.Unsigned
	var subnetID ids.ID
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		subnetID = unsignedTx.Subnet
	case *txs.AddSubnetValidatorTx:
		subnetID = unsignedTx.SubnetValidator.Subnet
	case *txs.CreateChainTx:
		subnetID = unsignedTx.SubnetID
	case *txs.TransformSubnetTx:
		subnetID = unsignedTx.Subnet
	case *txs.AddPermissionlessValidatorTx:
		subnetID = unsignedTx.Subnet
	case *txs.TransferSubnetOwnershipTx:
		subnetID = unsignedTx.Subnet
	default:
		return ids.Empty, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	return subnetID, nil
}

func (ms *Multisig) GetSubnetOwners() ([]ids.ShortID, uint32, error) {
	if ms.Undefined() {
		return nil, 0, ErrUndefinedTx
	}
	if ms.controlKeys == nil {
		subnetID, err := ms.GetSubnetID()
		if err != nil {
			return nil, 0, err
		}

		network, err := ms.GetNetwork()
		if err != nil {
			return nil, 0, err
		}
		controlKeys, threshold, err := GetOwners(network, subnetID)
		if err != nil {
			return nil, 0, err
		}
		ms.controlKeys = controlKeys
		ms.threshold = threshold
	}
	return ms.controlKeys, ms.threshold, nil
}

func GetOwners(network avalancheSDK.Network, subnetID ids.ID) ([]ids.ShortID, uint32, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx := context.Background()
	subnetResponse, err := pClient.GetSubnet(ctx, subnetID)
	if err != nil {
		return nil, 0, fmt.Errorf("subnet tx %s query error: %w", subnetID, err)
	}
	controlKeys := subnetResponse.ControlKeys
	threshold := subnetResponse.Threshold
	return controlKeys, threshold, nil
}

func (ms *Multisig) GetWrappedPChainTx() (*txs.Tx, error) {
	if ms.Undefined() {
		return nil, ErrUndefinedTx
	}
	return ms.PChainTx, nil
}

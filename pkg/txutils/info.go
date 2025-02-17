// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package txutils

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

// get network model associated to tx
func GetNetwork(tx *txs.Tx) (models.Network, error) {
	unsignedTx := tx.Unsigned
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
	case *txs.ConvertSubnetToL1Tx:
		networkID = unsignedTx.NetworkID
	default:
		return models.UndefinedNetwork, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	network := models.NetworkFromNetworkID(networkID)
	if network.Kind == models.Undefined {
		return models.UndefinedNetwork, fmt.Errorf("undefined network model for tx")
	}
	return network, nil
}

// get subnet id associated to tx
func GetSubnetID(tx *txs.Tx) (ids.ID, error) {
	unsignedTx := tx.Unsigned
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
	case *txs.ConvertSubnetToL1Tx:
		subnetID = unsignedTx.Subnet
	default:
		return ids.Empty, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	return subnetID, nil
}

func GetLedgerDisplayName(tx *txs.Tx) string {
	unsignedTx := tx.Unsigned
	switch unsignedTx.(type) {
	case *txs.AddSubnetValidatorTx:
		return "SubnetValidator"
	case *txs.CreateChainTx:
		return "CreateChain"
	default:
		return ""
	}
}

func IsCreateChainTx(tx *txs.Tx) bool {
	_, ok := tx.Unsigned.(*txs.CreateChainTx)
	return ok
}

func IsConvertToL1Tx(tx *txs.Tx) bool {
	_, ok := tx.Unsigned.(*txs.ConvertSubnetToL1Tx)
	return ok
}

func IsTransferSubnetOwnershipTx(tx *txs.Tx) bool {
	_, ok := tx.Unsigned.(*txs.TransferSubnetOwnershipTx)
	return ok
}

func GetOwners(network models.Network, subnetID ids.ID) (bool, []string, uint32, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx := context.Background()
	subnetResponse, err := pClient.GetSubnet(ctx, subnetID)
	if err != nil {
		return false, nil, 0, fmt.Errorf("subnet tx %s query error: %w", subnetID, err)
	}
	controlKeys := subnetResponse.ControlKeys
	threshold := subnetResponse.Threshold
	isPermissioned := subnetResponse.IsPermissioned
	hrp := key.GetHRP(network.ID)
	controlKeysStrs := []string{}
	for _, addr := range controlKeys {
		addrStr, err := address.Format("P", hrp, addr[:])
		if err != nil {
			return false, nil, 0, err
		}
		controlKeysStrs = append(controlKeysStrs, addrStr)
	}
	return isPermissioned, controlKeysStrs, threshold, nil
}

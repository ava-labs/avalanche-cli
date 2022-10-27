// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package txutils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

// get network model associated to tx
// expect tx.Unsigned type to be in [txs.AddSubnetValidatorTx, txs.CreateChainTx]
func GetNetwork(tx *txs.Tx) (models.Network, error) {
	unsignedTx := tx.Unsigned
	var networkID uint32
	switch unsignedTx := unsignedTx.(type) {
	case *txs.AddSubnetValidatorTx:
		networkID = unsignedTx.NetworkID
	case *txs.CreateChainTx:
		networkID = unsignedTx.NetworkID
	default:
		return models.Undefined, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	network := models.NetworkFromNetworkID(networkID)
	if network == models.Undefined {
		return models.Undefined, fmt.Errorf("undefined network model for tx")
	}
	return network, nil
}

func IsCreateChainTx(tx *txs.Tx) bool {
	_, ok := tx.Unsigned.(*txs.CreateChainTx)
	return ok
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"

	"github.com/ethereum/go-ethereum/common"
)

func validateConvertOperation(tx *txs.Tx, action string) (bool, error) {
	convertToL1Tx, ok := tx.Unsigned.(*txs.ConvertSubnetToL1Tx)
	if !ok {
		return false, fmt.Errorf("expected tx to be of type txs.ConvertSubnetToL1Tx, found %T", tx.Unsigned)
	}
	ux.Logger.PrintToUser("You are about to %s a txs.ConvertSubnetToL1Tx with the following content:", action)
	ux.Logger.PrintToUser("  Subnet ID: %s", convertToL1Tx.Subnet)
	ux.Logger.PrintToUser("  Blockchain ID: %s", convertToL1Tx.BlockchainID)
	ux.Logger.PrintToUser("  Manager Address: %s", common.BytesToAddress(convertToL1Tx.Address).Hex())
	ux.Logger.PrintToUser("  Validators:")
	for _, val := range convertToL1Tx.Validators {
		nodeID, err := ids.ToNodeID(val.NodeID)
		if err != nil {
			return false, fmt.Errorf("unexpected node ID on tx")
		}
		ux.Logger.PrintToUser("    %s", nodeID)
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Please review details and decide if it is safe to continue")
	ux.Logger.PrintToUser("")
	return app.Prompt.CaptureYesNo(fmt.Sprintf("Do you want to %s the transaction?", action))
}

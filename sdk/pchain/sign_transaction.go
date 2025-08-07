// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package pchain

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/sdk/constants"
	"github.com/ava-labs/avalanche-cli/sdk/multisig"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
)

func SignTransaction(
	client wallet.Wallet,
	tx *multisig.Multisig,
) error {
	ctx, cancel := utils.GetTimedContext(constants.SignatureTimeout)
	defer cancel()
	if err := client.P().Signer().Sign(ctx, tx.PChainTx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	return nil
}

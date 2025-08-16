// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package pchain

import (
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/multisig"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
)

func SendTransaction(
	client wallet.Wallet,
	tx *multisig.Multisig,
	awaitAcceptance bool,
) (string, error) {
	const (
		repeats             = 3
		sleepBetweenRepeats = 2 * time.Second
	)
	var err error
	for i := 0; i < repeats; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		options := []common.Option{common.WithContext(ctx)}
		if !awaitAcceptance {
			options = append(options, common.WithAssumeDecided())
		}
		err = client.P().IssueTx(tx.PChainTx, options...)
		if err == nil {
			break
		}
		time.Sleep(sleepBetweenRepeats)
	}
	return tx.PChainTx.ID().String(), err
}

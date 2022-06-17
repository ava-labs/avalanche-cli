// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/wallet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/vms/platformvm/validator"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/spf13/cobra"
)

var validatorStakeCmd = &cobra.Command{
	Use:   "stake [subnet]",
	Short: "stake a validator",
	Long:  ``,

	RunE:         stakeValidator,
	Args:         cobra.ExactArgs(0),
	SilenceUsage: true,
}

func stakeValidator(cmd *cobra.Command, args []string) error {
	uri := "https://api.avax-test.network"
	ctx := context.Background()
	keypath := "fabkey"
	netID := constants.FujiID
	sf, err := wallet.LoadSoft(netID, keypath)
	if err != nil {
		return err
	}
	kc := sf.KeyChain()
	walet, err := primary.NewWalletFromURI(ctx, uri, kc)
	if err != nil {
		return err
	}
	// TODO empty owner => no controlkeys
	rewardsOwner := &secp256k1fx.OutputOwners{}
	opts := []common.Option{}
	nodeID := ids.NodeID{}
	startTime := uint64(time.Now().Unix())
	endTime := uint64(time.Now().Unix())
	amount := uint64(42)
	validator := &validator.Validator{
		NodeID: nodeID,
		Start:  startTime,
		End:    endTime,
		Wght:   amount,
	}
	var shares uint32
	tx, err := walet.P().IssueAddValidatorTx(validator, rewardsOwner, shares, opts...)
	if err != nil {
		return err
	}
	fmt.Println(tx)
	return nil
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	minimumStakeAmountFlag     = "pos-minimum-stake-amount"
	maximumStakeAmountFlag     = "pos-maximum-stake-amount"
	minimumStakeDurationFlag   = "pos-minimum-stake-duration"
	minimumDelegationFeeFlag   = "pos-minimum-delegation-fee"
	maximumStakeMultiplierFlag = "pos-maximum-stake-multiplier"
	weightToValueFactorFlag    = "pos-weight-to-value-factor"
)

type POSFlags struct {
	MinimumStakeAmount     uint64
	MaximumStakeAmount     uint64
	MinimumStakeDuration   uint64
	MinimumDelegationFee   uint16
	MaximumStakeMultiplier uint8
	WeightToValueFactor    uint64
}

func AddProofOfStakeToCmd(cmd *cobra.Command, posFlags *POSFlags) GroupedFlags {
	return RegisterFlagGroup(cmd, "Proof Of Stake Flags", "show-pos-flags", false, func(set *pflag.FlagSet) {
		set.Uint64Var(&posFlags.MinimumStakeAmount, minimumStakeAmountFlag, 1, "minimum stake amount")
		set.Uint64Var(&posFlags.MaximumStakeAmount, maximumStakeAmountFlag, 1000, "maximum stake amount")
		set.Uint64Var(&posFlags.MinimumStakeDuration, minimumStakeDurationFlag, constants.PoSL1MinimumStakeDurationSeconds, "minimum stake duration (in seconds)")
		set.Uint16Var(&posFlags.MinimumDelegationFee, minimumDelegationFeeFlag, 1, "minimum delegation fee")
		set.Uint8Var(&posFlags.MaximumStakeMultiplier, maximumStakeMultiplierFlag, 1, "maximum stake multiplier")
		set.Uint64Var(&posFlags.WeightToValueFactor, weightToValueFactorFlag, 1, "weight to value factor")
	})
}

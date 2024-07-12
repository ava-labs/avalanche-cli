// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
)

func setFeeConfig(
	params SubnetEVMGenesisParams,
	config *params.ChainConfig,
) {
	config.FeeConfig = StarterFeeConfig

	switch {
	case params.feeConfig.highThroughput:
		config.FeeConfig.TargetGas = HighTarget
	case params.feeConfig.mediumThroughput:
		config.FeeConfig.TargetGas = MediumTarget
	case params.feeConfig.lowThroughput:
		config.FeeConfig.TargetGas = LowTarget
	default:
		setCustomFeeConfig(params, config)
	}
}

func setCustomFeeConfig(
	params SubnetEVMGenesisParams,
	config *params.ChainConfig,
) {
	config.FeeConfig = commontype.FeeConfig{
		GasLimit:                 params.feeConfig.gasLimit,
		TargetBlockRate:          params.feeConfig.blockRate.Uint64(),
		MinBaseFee:               params.feeConfig.minBaseFee,
		TargetGas:                params.feeConfig.targetGas,
		BaseFeeChangeDenominator: params.feeConfig.baseDenominator,
		MinBlockGasCost:          params.feeConfig.minBlockGas,
		MaxBlockGasCost:          params.feeConfig.maxBlockGas,
		BlockGasCostStep:         params.feeConfig.gasStep,
	}
}

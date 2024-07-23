// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
)

func setStandardGas(
	params SubnetEVMGenesisParams,
	config *params.ChainConfig,
	gasLimit *big.Int,
	targetGas *big.Int,
	useDynamicFees bool,
) {
	config.FeeConfig.GasLimit = gasLimit
	config.FeeConfig.TargetGas = targetGas
	if useDynamicFees {
		config.FeeConfig.TargetGas = config.FeeConfig.TargetGas.Mul(config.FeeConfig.GasLimit, NoDynamicFeesGasLimitToTargetGasFactor)
	}
}

func setFeeConfig(
	params SubnetEVMGenesisParams,
	config *params.ChainConfig,
) {
	config.FeeConfig = StarterFeeConfig

	switch {
	case params.feeConfig.lowThroughput:
		setStandardGas(params, config, LowGasLimit, LowTargetGas, params.feeConfig.useDynamicFees)
	case params.feeConfig.mediumThroughput:
		setStandardGas(params, config, MediumGasLimit, MediumTargetGas, params.feeConfig.useDynamicFees)
	case params.feeConfig.highThroughput:
		setStandardGas(params, config, HighGasLimit, HighTargetGas, params.feeConfig.useDynamicFees)
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

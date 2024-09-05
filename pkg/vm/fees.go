// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/subnet-evm/commontype"
)

func SetStandardGas(
	feeConfig *commontype.FeeConfig,
	gasLimit *big.Int,
	targetGas *big.Int,
	useDynamicFees bool,
) {
	feeConfig.GasLimit = gasLimit
	feeConfig.TargetGas = targetGas
	if !useDynamicFees {
		feeConfig.TargetGas = feeConfig.TargetGas.Mul(feeConfig.GasLimit, NoDynamicFeesGasLimitToTargetGasFactor)
	}
}

func getFeeConfig(
	params SubnetEVMGenesisParams,
) commontype.FeeConfig {
	feeConfig := StarterFeeConfig
	switch {
	case params.feeConfig.lowThroughput:
		SetStandardGas(&feeConfig, LowGasLimit, LowTargetGas, params.feeConfig.useDynamicFees)
	case params.feeConfig.mediumThroughput:
		SetStandardGas(&feeConfig, MediumGasLimit, MediumTargetGas, params.feeConfig.useDynamicFees)
	case params.feeConfig.highThroughput:
		SetStandardGas(&feeConfig, HighGasLimit, HighTargetGas, params.feeConfig.useDynamicFees)
	default:
		feeConfig = getCustomFeeConfig(params)
	}
	fmt.Printf("obtained gas limit LowGasLimit %s LowTargetGas %s \n", feeConfig.GasLimit, feeConfig.TargetGas)
	return feeConfig
}

func getCustomFeeConfig(
	params SubnetEVMGenesisParams,
) commontype.FeeConfig {
	return commontype.FeeConfig{
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

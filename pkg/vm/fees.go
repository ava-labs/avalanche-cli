// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/params"
)

func getFeeConfig(config params.ChainConfig) (params.ChainConfig, stateDirection, error) {
	const (
		useFast   = "High disk use   / High Throughput   5 mil   gas/s"
		useMedium = "Medium disk use / Medium Throughput 2 mil   gas/s"
		useSlow   = "Low disk use    / Low Throughput    1.5 mil gas/s (C-Chain's setting)"
		customFee = "Customize fee config"

		setGasLimit                 = "Set gas limit"
		setBlockRate                = "Set target block rate"
		setMinBaseFee               = "Set min base fee"
		setTargetGas                = "Set target gas"
		setBaseFeeChangeDenominator = "Set base fee change denominator"
		setMinBlockGas              = "Set min block gas cost"
		setMaxBlockGas              = "Set max block gas cost"
		setGasStep                  = "Set block gas cost step"
	)

	feeConfigOptions := []string{useSlow, useMedium, useFast, customFee, goBackMsg}

	feeDefault, err := prompts.CaptureList(
		"How would you like to set fees",
		feeConfigOptions,
	)
	if err != nil {
		return config, stop, err
	}

	config.FeeConfig = StarterFeeConfig

	switch feeDefault {
	case useFast:
		config.FeeConfig.TargetGas = fastTarget
		return config, forward, nil
	case useMedium:
		config.FeeConfig.TargetGas = mediumTarget
		return config, forward, nil
	case useSlow:
		config.FeeConfig.TargetGas = slowTarget
		return config, forward, nil
	case goBackMsg:
		return config, backward, nil
	default:
		ux.Logger.PrintToUser("Customizing fee config")
	}

	gasLimit, err := prompts.CapturePositiveBigInt(setGasLimit)
	if err != nil {
		return config, stop, err
	}

	blockRate, err := prompts.CapturePositiveBigInt(setBlockRate)
	if err != nil {
		return config, stop, err
	}

	minBaseFee, err := prompts.CapturePositiveBigInt(setMinBaseFee)
	if err != nil {
		return config, stop, err
	}

	targetGas, err := prompts.CapturePositiveBigInt(setTargetGas)
	if err != nil {
		return config, stop, err
	}

	baseDenominator, err := prompts.CapturePositiveBigInt(setBaseFeeChangeDenominator)
	if err != nil {
		return config, stop, err
	}

	minBlockGas, err := prompts.CapturePositiveBigInt(setMinBlockGas)
	if err != nil {
		return config, stop, err
	}

	maxBlockGas, err := prompts.CapturePositiveBigInt(setMaxBlockGas)
	if err != nil {
		return config, stop, err
	}

	gasStep, err := prompts.CapturePositiveBigInt(setGasStep)
	if err != nil {
		return config, stop, err
	}

	feeConf := params.FeeConfig{
		GasLimit:                 gasLimit,
		TargetBlockRate:          blockRate.Uint64(),
		MinBaseFee:               minBaseFee,
		TargetGas:                targetGas,
		BaseFeeChangeDenominator: baseDenominator,
		MinBlockGasCost:          minBlockGas,
		MaxBlockGasCost:          maxBlockGas,
		BlockGasCostStep:         gasStep,
	}

	config.FeeConfig = feeConf

	return config, forward, nil
}

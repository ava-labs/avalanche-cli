// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ava-labs/subnet-evm/params"
)

func GetFeeConfig(config params.ChainConfig, app *application.Avalanche, useDefault bool) (
	params.ChainConfig,
	statemachine.StateDirection,
	error,
) {
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

	config.FeeConfig = StarterFeeConfig

	if useDefault {
		config.FeeConfig.TargetGas = slowTarget
		return config, statemachine.Forward, nil
	}

	feeConfigOptions := []string{useSlow, useMedium, useFast, customFee, goBackMsg}

	feeDefault, err := app.Prompt.CaptureList(
		"How would you like to set fees",
		feeConfigOptions,
	)
	if err != nil {
		return config, statemachine.Stop, err
	}

	switch feeDefault {
	case useFast:
		config.FeeConfig.TargetGas = fastTarget
		return config, statemachine.Forward, nil
	case useMedium:
		config.FeeConfig.TargetGas = mediumTarget
		return config, statemachine.Forward, nil
	case useSlow:
		config.FeeConfig.TargetGas = slowTarget
		return config, statemachine.Forward, nil
	case goBackMsg:
		return config, statemachine.Backward, nil
	default:
		ux.Logger.PrintToUser("Customizing fee config")
	}

	gasLimit, err := app.Prompt.CapturePositiveBigInt(setGasLimit)
	if err != nil {
		return config, statemachine.Stop, err
	}

	blockRate, err := app.Prompt.CapturePositiveBigInt(setBlockRate)
	if err != nil {
		return config, statemachine.Stop, err
	}

	minBaseFee, err := app.Prompt.CapturePositiveBigInt(setMinBaseFee)
	if err != nil {
		return config, statemachine.Stop, err
	}

	targetGas, err := app.Prompt.CapturePositiveBigInt(setTargetGas)
	if err != nil {
		return config, statemachine.Stop, err
	}

	baseDenominator, err := app.Prompt.CapturePositiveBigInt(setBaseFeeChangeDenominator)
	if err != nil {
		return config, statemachine.Stop, err
	}

	minBlockGas, err := app.Prompt.CapturePositiveBigInt(setMinBlockGas)
	if err != nil {
		return config, statemachine.Stop, err
	}

	maxBlockGas, err := app.Prompt.CapturePositiveBigInt(setMaxBlockGas)
	if err != nil {
		return config, statemachine.Stop, err
	}

	gasStep, err := app.Prompt.CapturePositiveBigInt(setGasStep)
	if err != nil {
		return config, statemachine.Stop, err
	}

	feeConf := commontype.FeeConfig{
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

	return config, statemachine.Forward, nil
}

package vm

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/subnet-evm/params"
)

const stageAfterFees = airdropStage
const stageBeforeFees = descriptorStage

func getFeeConfig(config params.ChainConfig) (params.ChainConfig, creationStage, error) {
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
		return config, errored, err
	}

	switch feeDefault {
	case useFast:
		StarterFeeConfig.TargetGas = fastTarget
		config.FeeConfig = StarterFeeConfig
		return config, stageAfterFees, nil
	case useMedium:
		StarterFeeConfig.TargetGas = mediumTarget
		config.FeeConfig = StarterFeeConfig
		return config, stageAfterFees, nil
	case useSlow:
		StarterFeeConfig.TargetGas = slowTarget
		config.FeeConfig = StarterFeeConfig
		return config, stageAfterFees, nil
	case goBackMsg:
		return config, stageBeforeFees, nil
	default:
		fmt.Println("Customizing fee config")
	}

	gasLimit, err := prompts.CapturePositiveBigInt(setGasLimit)
	if err != nil {
		return config, errored, err
	}

	blockRate, err := prompts.CapturePositiveBigInt(setBlockRate)
	if err != nil {
		return config, errored, err
	}

	minBaseFee, err := prompts.CapturePositiveBigInt(setMinBaseFee)
	if err != nil {
		return config, errored, err
	}

	targetGas, err := prompts.CapturePositiveBigInt(setTargetGas)
	if err != nil {
		return config, errored, err
	}

	baseDenominator, err := prompts.CapturePositiveBigInt(setBaseFeeChangeDenominator)
	if err != nil {
		return config, errored, err
	}

	minBlockGas, err := prompts.CapturePositiveBigInt(setMinBlockGas)
	if err != nil {
		return config, errored, err
	}

	maxBlockGas, err := prompts.CapturePositiveBigInt(setMaxBlockGas)
	if err != nil {
		return config, errored, err
	}

	gasStep, err := prompts.CapturePositiveBigInt(setGasStep)
	if err != nil {
		return config, errored, err
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

	return config, stageAfterFees, nil
}

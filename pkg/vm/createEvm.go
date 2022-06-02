// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
)

func CreateEvmGenesis(name string, log logging.Logger) ([]byte, error) {
	ux.Logger.PrintToUser("creating subnet %s", name)

	genesis := core.Genesis{}
	conf := params.SubnetEVMDefaultChainConfig

	chainId, err := getChainId()
	if err != nil {
		return []byte{}, err
	}
	conf.ChainID = chainId

	*conf, err = getFeeConfig(*conf)
	if err != nil {
		return []byte{}, err
	}

	allocation, err := getAllocation()
	if err != nil {
		return []byte{}, err
	}

	*conf, err = getPrecompiles(*conf)
	if err != nil {
		return []byte{}, err
	}

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = GasLimit

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return []byte{}, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return prettyJSON.Bytes(), nil
}

func getChainId() (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	fmt.Println("Select your subnet's ChainId. It can be any positive integer.")

	chainId, err := prompts.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	return chainId, nil
}

func getAllocation() (core.GenesisAlloc, error) {
	first := true

	allocation := core.GenesisAlloc{}

	for {
		firstStr := "Would you like to airdrop tokens?"
		secondStr := "Would you like to airdrop more tokens?"

		var promptStr string
		if promptStr = secondStr; first {
			promptStr = firstStr
			first = false
		}

		continueAirdrop, err := prompts.CaptureYesNo(promptStr)
		if err != nil {
			return nil, err
		}

		if continueAirdrop {
			addressHex, err := prompts.CaptureAddress("Address")
			if err != nil {
				return nil, err
			}

			amount, err := prompts.CapturePositiveBigInt("Amount (in wei)")
			if err != nil {
				return nil, err
			}

			account := core.GenesisAccount{
				Balance: amount,
			}

			allocation[addressHex] = account

		} else {
			return allocation, nil
		}
	}
}

func getFeeConfig(config params.ChainConfig) (params.ChainConfig, error) {
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

	feeConfigOptions := []string{useSlow, useMedium, useFast, customFee}

	feeDefault, err := prompts.CaptureList(
		"How would you like to set fees",
		feeConfigOptions,
	)
	if err != nil {
		return config, err
	}

	switch feeDefault {
	case useFast:
		StarterFeeConfig.TargetGas = fastTarget
		config.FeeConfig = StarterFeeConfig
		return config, nil
	case useMedium:
		StarterFeeConfig.TargetGas = mediumTarget
		config.FeeConfig = StarterFeeConfig
		return config, nil
	case useSlow:
		StarterFeeConfig.TargetGas = slowTarget
		config.FeeConfig = StarterFeeConfig
		return config, nil
	default:
		fmt.Println("Customizing fee config")
	}

	gasLimit, err := prompts.CapturePositiveBigInt(setGasLimit)
	if err != nil {
		return config, err
	}

	blockRate, err := prompts.CapturePositiveBigInt(setBlockRate)
	if err != nil {
		return config, err
	}

	minBaseFee, err := prompts.CapturePositiveBigInt(setMinBaseFee)
	if err != nil {
		return config, err
	}

	targetGas, err := prompts.CapturePositiveBigInt(setTargetGas)
	if err != nil {
		return config, err
	}

	baseDenominator, err := prompts.CapturePositiveBigInt(setBaseFeeChangeDenominator)
	if err != nil {
		return config, err
	}

	minBlockGas, err := prompts.CapturePositiveBigInt(setMinBlockGas)
	if err != nil {
		return config, err
	}

	maxBlockGas, err := prompts.CapturePositiveBigInt(setMaxBlockGas)
	if err != nil {
		return config, err
	}

	gasStep, err := prompts.CapturePositiveBigInt(setGasStep)
	if err != nil {
		return config, err
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

	return config, nil
}

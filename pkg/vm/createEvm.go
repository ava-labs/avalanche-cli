/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
)

var WagmiFeeConfig = params.FeeConfig{
	GasLimit:                 big.NewInt(20000000),
	MinBaseFee:               big.NewInt(1000000000),
	TargetGas:                big.NewInt(100000000),
	BaseFeeChangeDenominator: big.NewInt(48),
	MinBlockGasCost:          big.NewInt(0),
	MaxBlockGasCost:          big.NewInt(10000000),
	TargetBlockRate:          2,
	BlockGasCostStep:         big.NewInt(500000),
}

var CChainFeeConfig = params.FeeConfig{
	GasLimit:                 big.NewInt(20000000),
	MinBaseFee:               big.NewInt(1000000000),
	TargetGas:                big.NewInt(100000000),
	BaseFeeChangeDenominator: big.NewInt(48),
	MinBlockGasCost:          big.NewInt(0),
	MaxBlockGasCost:          big.NewInt(10000000),
	TargetBlockRate:          2,
	BlockGasCostStep:         big.NewInt(500000),
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
		if promptStr = secondStr; first == true {
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

func configureContractAllowList() (precompile.ContractDeployerAllowListConfig, error) {
	addAdmin := "Add admin"
	preview := "Preview"
	moreInfo := "More info"
	doneMsg := "Done"

	config := precompile.ContractDeployerAllowListConfig{}
	allowList := precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: []common.Address{},
	}

	for {
		listDecision, err := prompts.CaptureList(
			"Configure contract deployment allow list:",
			[]string{addAdmin, preview, moreInfo, doneMsg},
		)
		if err != nil {
			return config, err
		}

		switch listDecision {
		case addAdmin:
			adminAddr, err := prompts.CaptureAddress("Admin Address")
			if err != nil {
				return config, err
			}
			allowList.AllowListAdmins = append(allowList.AllowListAdmins, adminAddr)
		case preview:
			fmt.Println("Admins:")
			for i, addr := range allowList.AllowListAdmins {
				fmt.Printf("%d. %s\n", i, addr.Hex())
			}
		case doneMsg:
			config.AllowListConfig = allowList
			return config, nil
		case moreInfo:
			fmt.Printf("\nThis precompile restricts who has the ability to deploy contracts " +
				"on your subnet.\nFor more information visit https://github.com/ava-labs/subnet-" +
				"evm#restricting-smart-contract-deployers.\n\n")
		default:
			return config, errors.New("Unexpected option")
		}
	}
}

func removePrecompile(arr []string, s string) ([]string, error) {
	for i, val := range arr {
		if val == s {
			return append(arr[:i], arr[i+1:]...), nil
		}
	}
	return arr, errors.New("String not in array")
}

func getFeeConfig(config params.ChainConfig) (params.ChainConfig, error) {
	const useWagmi = "Use WAGMI defaults"
	const useCChain = "Use C-Chain defaults"
	const customFee = "Customize fee config"

	const setGasLimit = "Set gas limit"
	const setBlockRate = "Set target block rate"
	const setMinBaseFee = "Set min base fee"
	const setTargetGas = "Set target gas"
	const setBaseFeeChangeDenominator = "Set base fee change denominator"
	const setMinBlockGas = "Set min block gas cost"
	const setMaxBlockGas = "Set max block gas cost"
	const setGasStep = "Set block gas cost step"

	feeConfigOptions := []string{useWagmi, useCChain, customFee}

	feeDefault, err := prompts.CaptureList(
		"How would you like to set fees",
		feeConfigOptions,
	)
	if err != nil {
		return config, err
	}

	switch feeDefault {
	case useWagmi:
		fmt.Println("Using Wagmi config")
		config.FeeConfig = &WagmiFeeConfig
		return config, nil
	case useCChain:
		fmt.Println("Using C-Chain config")
		config.FeeConfig = &CChainFeeConfig
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

	config.FeeConfig = &feeConf

	return config, nil
}

func getPrecompiles(config params.ChainConfig) (params.ChainConfig, error) {
	const nativeMint = "Native Minting"
	const contractAllowList = "Contract deployment whitelist"
	const txAllowList = "Transaction allow list"
	const cancel = "Cancel"

	first := true

	remainingPrecompiles := []string{nativeMint, contractAllowList, txAllowList, cancel}

	for {
		firstStr := "Would you like to add a custom precompile?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first == true {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := prompts.CaptureYesNo(promptStr)
		if err != nil {
			return config, err
		}

		if addPrecompile {
			precompileDecision, err := prompts.CaptureList(
				"Choose precompile:",
				remainingPrecompiles,
			)
			if err != nil {
				return config, err
			}

			switch precompileDecision {
			case nativeMint:
				fmt.Println("TODO")
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, nativeMint)
				if err != nil {
					return config, err
				}
			case contractAllowList:
				contractConfig, err := configureContractAllowList()
				if err != nil {
					return config, err
				}
				config.ContractDeployerAllowListConfig = contractConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, contractAllowList)
				if err != nil {
					return config, err
				}
			case txAllowList:
				fmt.Println("TODO")
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, txAllowList)
				if err != nil {
					return config, err
				}
			case cancel:
				return config, nil
			}

			if len(remainingPrecompiles) == 1 {
				return config, nil
			}

		} else {
			return config, nil
		}
	}
}

func CreateEvmGenesis(name string) ([]byte, error) {
	fmt.Println("creating subnet", name)

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

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return []byte{}, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	fmt.Println(string(prettyJSON.Bytes()))
	return prettyJSON.Bytes(), nil
}

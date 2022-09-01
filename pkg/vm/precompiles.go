// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile"
	"github.com/ethereum/go-ethereum/common"
)

func getAdminList(initialPrompt string, info string, app *application.Avalanche) ([]common.Address, bool, error) {
	label := "Address"

	list, canceled, err := app.Prompt.CaptureListDecision(
		app.Prompt,
		initialPrompt,
		app.Prompt.CaptureAddress,
		"Enter Address ",
		label,
		info,
		nil,
	)

	admins := make([]common.Address, len(list))
	var (
		addr common.Address
		ok   bool
	)
	for i, a := range list {
		if addr, ok = a.(common.Address); !ok {
			return nil, false, fmt.Errorf("expected common.Address but got %T", addr)
		}
		admins[i] = addr
	}

	return admins, canceled, err
}

func configureContractAllowList(app *application.Avalanche) (precompile.ContractDeployerAllowListConfig, bool, error) {
	config := precompile.ContractDeployerAllowListConfig{}
	prompt := "Configure contract deployment allow list"
	info := "\nThis precompile restricts who has the ability to deploy contracts " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-smart-contract-deployers\n\n"

	admins, cancelled, err := getAdminList(prompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func configureTransactionAllowList(app *application.Avalanche) (precompile.TxAllowListConfig, bool, error) {
	config := precompile.TxAllowListConfig{}
	prompt := "Configure transaction allow list"
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n\n"

	admins, cancelled, err := getAdminList(prompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func configureMinterList(app *application.Avalanche) (precompile.ContractNativeMinterConfig, bool, error) {
	config := precompile.ContractNativeMinterConfig{}
	prompt := "Configure native minting allow list"
	info := "\nThis precompile allows admins to permit designated contracts to mint the native token " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#minting-native-coins\n\n"

	admins, cancelled, err := getAdminList(prompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func configureFeeConfigAllowList(app *application.Avalanche) (precompile.FeeConfigManagerConfig, bool, error) {
	config := precompile.FeeConfigManagerConfig{}
	prompt := "Configure fee manager allow list"
	info := "\nThis precompile allows admins to adjust chain gas and fee parameters without " +
		"performing a hardfork.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#configuring-dynamic-fees\n\n"

	admins, cancelled, err := getAdminList(prompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = precompile.AllowListConfig{
		BlockTimestamp:  big.NewInt(0),
		AllowListAdmins: admins,
	}

	return config, cancelled, nil
}

func removePrecompile(arr []string, s string) ([]string, error) {
	for i, val := range arr {
		if val == s {
			return append(arr[:i], arr[i+1:]...), nil
		}
	}
	return arr, errors.New("string not in array")
}

func getPrecompiles(config params.ChainConfig, app *application.Avalanche) (
	params.ChainConfig,
	statemachine.StateDirection,
	error,
) {
	const (
		nativeMint        = "Native Minting"
		contractAllowList = "Contract Deployment Allow List"
		txAllowList       = "Transaction Allow List"
		feeManager        = "Manage Fee Settings"
		cancel            = "Cancel"
	)

	first := true

	remainingPrecompiles := []string{nativeMint, contractAllowList, txAllowList, feeManager, cancel}

	for {
		firstStr := "Advanced: Would you like to add a custom precompile to modify the EVM?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := app.Prompt.CaptureList(promptStr, []string{prompts.No, prompts.Yes, goBackMsg})
		if err != nil {
			return config, statemachine.Stop, err
		}

		switch addPrecompile {
		case prompts.No:
			return config, statemachine.Forward, nil
		case goBackMsg:
			return config, statemachine.Backward, nil
		}

		precompileDecision, err := app.Prompt.CaptureList(
			"Choose precompile",
			remainingPrecompiles,
		)
		if err != nil {
			return config, statemachine.Stop, err
		}

		switch precompileDecision {
		case nativeMint:
			mintConfig, cancelled, err := configureMinterList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.ContractNativeMinterConfig = mintConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, nativeMint)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case contractAllowList:
			contractConfig, cancelled, err := configureContractAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.ContractDeployerAllowListConfig = contractConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, contractAllowList)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case txAllowList:
			txConfig, cancelled, err := configureTransactionAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.TxAllowListConfig = txConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, txAllowList)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case feeManager:
			feeConfig, cancelled, err := configureFeeConfigAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.FeeManagerConfig = feeConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, feeManager)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case cancel:
			return config, statemachine.Forward, nil
		}

		// When all precompiles have been added, the len of remainingPrecompiles will be 1
		// (the cancel option stays in the list). Safe to return.
		if len(remainingPrecompiles) == 1 {
			return config, statemachine.Forward, nil
		}
	}
}

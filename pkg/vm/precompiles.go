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
	"github.com/ava-labs/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/feemanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ava-labs/subnet-evm/precompile/contracts/rewardmanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	"github.com/ethereum/go-ethereum/common"
)

type Precompile string

const (
	NativeMint        = "Native Minting"
	ContractAllowList = "Contract Deployment Allow List"
	TxAllowList       = "Transaction Allow List"
	FeeManager        = "Manage Fee Settings"
	RewardManager     = "RewardManagerConfig"
)

func PrecompileToUpgradeString(p Precompile) string {
	switch p {
	case NativeMint:
		return "contractNativeMinterConfig"
	case ContractAllowList:
		return "contractDeployerAllowListConfig"
	case TxAllowList:
		return "txAllowListConfig"
	case FeeManager:
		return "feeManagerConfig"
	case RewardManager:
		return "rewardManagerConfig"
	default:
		return ""
	}
}

func configureRewardManager(app *application.Avalanche) (rewardmanager.Config, bool, error) {
	config := rewardmanager.Config{}
	adminPrompt := "Configure reward manager admins"
	enabledPrompt := "Configure reward manager enabled addresses"
	info := "\nThis precompile allows to configure the fee reward mechanism " +
		"on your subnet, including burning or sending fees.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#changing-fee-reward-mechanisms\n\n"

	admins, enabled, cancelled, err := getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: big.NewInt(0),
	}
	config.InitialRewardConfig, err = ConfigureInitialRewardConfig(app)
	if err != nil {
		return config, false, err
	}

	return config, cancelled, nil
}

func ConfigureInitialRewardConfig(app *application.Avalanche) (*rewardmanager.InitialRewardConfig, error) {
	config := &rewardmanager.InitialRewardConfig{}

	burnPrompt := "Should fees be burnt?"
	burnFees, err := app.Prompt.CaptureYesNo(burnPrompt)
	if err != nil {
		return config, err
	}
	if burnFees {
		return config, nil
	}

	feeRcpdPrompt := "Allow block producers to claim fees?"
	allowFeeRecipients, err := app.Prompt.CaptureYesNo(feeRcpdPrompt)
	if err != nil {
		return config, err
	}
	if allowFeeRecipients {
		config.AllowFeeRecipients = true
		return config, nil
	}

	rewardPrompt := "Provide the address to which fees will be sent to"
	rewardAddress, err := app.Prompt.CaptureAddress(rewardPrompt)
	if err != nil {
		return config, err
	}
	config.RewardAddress = rewardAddress
	return config, nil
}

func getAddressList(initialPrompt string, info string, app *application.Avalanche) ([]common.Address, bool, error) {
	label := "Address"

	return prompts.CaptureListDecision(
		app.Prompt,
		initialPrompt,
		app.Prompt.CaptureAddress,
		"Enter Address ",
		label,
		info,
	)
}

func configureContractAllowList(app *application.Avalanche) (deployerallowlist.Config, bool, error) {
	config := deployerallowlist.Config{}
	adminPrompt := "Configure contract deployment admin allow list"
	enabledPrompt := "Configure contract deployment enabled addresses list"
	info := "\nThis precompile restricts who has the ability to deploy contracts " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-smart-contract-deployers\n\n"

	admins, enabled, cancelled, err := getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: big.NewInt(0),
	}

	return config, cancelled, nil
}

func configureTransactionAllowList(app *application.Avalanche) (txallowlist.Config, bool, error) {
	config := txallowlist.Config{}
	adminPrompt := "Configure transaction allow list admin addresses"
	enabledPrompt := "Configure transaction allow list enabled addresses"
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n\n"

	admins, enabled, cancelled, err := getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: big.NewInt(0),
	}

	return config, cancelled, nil
}

func getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info string, app *application.Avalanche) ([]common.Address, []common.Address, bool, error) {
	admins, cancelled, err := getAddressList(adminPrompt, info, app)
	if err != nil || cancelled {
		return nil, nil, false, err
	}
	adminsMap := make(map[string]bool)
	for _, adminsAddress := range admins {
		adminsMap[adminsAddress.String()] = true
	}
	enabled, cancelled, err := getAddressList(enabledPrompt, info, app)
	if err != nil {
		return nil, nil, false, err
	}
	for _, enabledAddress := range enabled {
		if _, ok := adminsMap[enabledAddress.String()]; ok {
			return nil, nil, false, fmt.Errorf("Can't have address %s in both admin and enabled addresses", enabledAddress.String())
		}
	}
	return admins, enabled, cancelled, nil
}

func configureMinterList(app *application.Avalanche) (nativeminter.Config, bool, error) {
	config := nativeminter.Config{}
	adminPrompt := "Configure native minting allow list"
	enabledPrompt := "Configure native minting enabled addresses"
	info := "\nThis precompile allows admins to permit designated contracts to mint the native token " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#minting-native-coins\n\n"

	admins, enabled, cancelled, err := getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info, app)
	if err != nil {
		return config, false, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: big.NewInt(0),
	}

	return config, cancelled, nil
}

func configureFeeConfigAllowList(app *application.Avalanche) (feemanager.Config, bool, error) {
	config := feemanager.Config{}
	adminPrompt := "Configure fee manager allow list"
	enabledPrompt := "Configure native minting enabled addresses"
	info := "\nThis precompile allows admins to adjust chain gas and fee parameters without " +
		"performing a hardfork.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#configuring-dynamic-fees\n\n"

	admins, enabled, cancelled, err := getAdminAndEnabledAddresses(adminPrompt, enabledPrompt, info, app)
	if err != nil {
		return config, false, err
	}

	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   admins,
		EnabledAddresses: enabled,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: big.NewInt(0),
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
	const cancel = "Cancel"

	first := true

	remainingPrecompiles := []string{NativeMint, ContractAllowList, TxAllowList, FeeManager, RewardManager, cancel}

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
		case NativeMint:
			mintConfig, cancelled, err := configureMinterList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[nativeminter.ConfigKey] = &mintConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, NativeMint)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case ContractAllowList:
			contractConfig, cancelled, err := configureContractAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[deployerallowlist.ConfigKey] = &contractConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, ContractAllowList)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case TxAllowList:
			txConfig, cancelled, err := configureTransactionAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[txallowlist.ConfigKey] = &txConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, TxAllowList)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case FeeManager:
			feeConfig, cancelled, err := configureFeeConfigAllowList(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[feemanager.ConfigKey] = &feeConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, FeeManager)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case RewardManager:
			rewardManagerConfig, cancelled, err := configureRewardManager(app)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[rewardmanager.ConfigKey] = &rewardManagerConfig
				remainingPrecompiles, err = removePrecompile(remainingPrecompiles, RewardManager)
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

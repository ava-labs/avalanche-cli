// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/feemanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ava-labs/subnet-evm/precompile/contracts/rewardmanager"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	subnetevmutils "github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
)

type Precompile string

const (
	NativeMint        = "Native Minting"
	ContractAllowList = "Contract Deployment Allow List"
	TxAllowList       = "Transaction Allow List"
	FeeManager        = "Adjust Fee Settings Post Deploy"
	RewardManager     = "Customize Fees Distribution"
	Warp              = "Warp"
)

func configureContractAllowList(
	app *application.Avalanche,
	subnetEvmVersion string,
) (deployerallowlist.Config, bool, error) {
	config := deployerallowlist.Config{}
	info := "\nThis precompile restricts who has the ability to deploy contracts " +
		"on your subnet.\nFor more information visit " + //nolint:goconst
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-smart-contract-deployers\n"
	ux.Logger.PrintToUser(info)
	allowList, cancelled, err := GenerateAllowList(app, "deploy smart contracts", subnetEvmVersion)
	if cancelled || err != nil {
		return config, cancelled, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   allowList.AdminAddresses,
		ManagerAddresses: allowList.ManagerAddresses,
		EnabledAddresses: allowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config, cancelled, nil
}

func configureTransactionAllowList(
	app *application.Avalanche,
	subnetEvmVersion string,
) (txallowlist.Config, bool, error) {
	config := txallowlist.Config{}
	info := "\nThis precompile restricts who has the ability to issue transactions " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet/#restricting-who-can-submit-transactions\n"
	ux.Logger.PrintToUser(info)
	allowList, cancelled, err := GenerateAllowList(app, "issue transactions", subnetEvmVersion)
	if cancelled || err != nil {
		return config, cancelled, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   allowList.AdminAddresses,
		ManagerAddresses: allowList.ManagerAddresses,
		EnabledAddresses: allowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config, cancelled, nil
}

func configureMinterList(
	app *application.Avalanche,
	subnetEvmVersion string,
) (nativeminter.Config, bool, error) {
	config := nativeminter.Config{}
	info := "\nThis precompile allows admins to permit designated contracts to mint the native token " +
		"on your subnet.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#minting-native-coins\n"
	ux.Logger.PrintToUser(info)
	allowList, cancelled, err := GenerateAllowList(app, "mint native tokens", subnetEvmVersion)
	if cancelled || err != nil {
		return config, cancelled, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   allowList.AdminAddresses,
		ManagerAddresses: allowList.ManagerAddresses,
		EnabledAddresses: allowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config, cancelled, nil
}

func configureFeeConfigAllowList(
	app *application.Avalanche,
	subnetEvmVersion string,
) (feemanager.Config, bool, error) {
	config := feemanager.Config{}
	info := "\nThis precompile allows admins to adjust chain gas and fee parameters without " +
		"performing a hardfork.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#configuring-dynamic-fees\n"
	ux.Logger.PrintToUser(info)
	allowList, cancelled, err := GenerateAllowList(app, "adjust the gas fees", subnetEvmVersion)
	if cancelled || err != nil {
		return config, cancelled, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   allowList.AdminAddresses,
		ManagerAddresses: allowList.ManagerAddresses,
		EnabledAddresses: allowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config, cancelled, nil
}

func configureRewardManager(
	app *application.Avalanche,
	subnetEvmVersion string,
) (rewardmanager.Config, bool, error) {
	info := "\nThis precompile allows to configure the fee reward mechanism " +
		"on your subnet, including burning or sending fees.\nFor more information visit " +
		"https://docs.avax.network/subnets/customize-a-subnet#changing-fee-reward-mechanisms\n"
	ux.Logger.PrintToUser(info)
	config := rewardmanager.Config{}
	allowList, cancelled, err := GenerateAllowList(app, "customize fee distribution", subnetEvmVersion)
	if cancelled || err != nil {
		return config, cancelled, err
	}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   allowList.AdminAddresses,
		ManagerAddresses: allowList.ManagerAddresses,
		EnabledAddresses: allowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	config.InitialRewardConfig, err = ConfigureInitialRewardConfig(app)
	return config, cancelled, err
}

func ConfigureInitialRewardConfig(
	app *application.Avalanche,
) (*rewardmanager.InitialRewardConfig, error) {
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

func configureWarp(timestamp *uint64) warp.Config {
	config := warp.Config{
		QuorumNumerator: warp.WarpDefaultQuorumNumerator,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: timestamp,
	}
	return config
}

func removePrecompile(arr []string, s string) ([]string, error) {
	for i, val := range arr {
		if val == s {
			return append(arr[:i], arr[i+1:]...), nil
		}
	}
	return arr, errors.New("string not in array")
}

// adds teleporter-related addresses (main funded key, messenger deploy key, relayer key)
// to the allow list of relevant enabled precompiles
func addTeleporterAddressesToAllowLists(
	config params.ChainConfig,
	teleporterAddress string,
	teleporterMessengerDeployerAddress string,
	relayerAddress string,
) params.ChainConfig {
	// tx allow list:
	// teleporterAddress funds the other two and also deploys the registry
	// teleporterMessengerDeployerAddress deploys the messenger
	// relayerAddress is used by the relayer to send txs to the target chain
	for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress, relayerAddress} {
		precompileConfig := config.GenesisPrecompiles[txallowlist.ConfigKey]
		if precompileConfig != nil {
			txAllowListConfig := precompileConfig.(*txallowlist.Config)
			txAllowListConfig.AllowListConfig = addAddressToAllowed(
				txAllowListConfig.AllowListConfig,
				address,
			)
		}
	}
	// contract deploy allow list:
	// teleporterAddress deploys the registry
	// teleporterMessengerDeployerAddress deploys the messenger
	for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress} {
		precompileConfig := config.GenesisPrecompiles[deployerallowlist.ConfigKey]
		if precompileConfig != nil {
			txAllowListConfig := precompileConfig.(*deployerallowlist.Config)
			txAllowListConfig.AllowListConfig = addAddressToAllowed(
				txAllowListConfig.AllowListConfig,
				address,
			)
		}
	}
	return config
}

// adds an address to the given allowlist, as an Allowed address,
// if it is not yet Admin, Manager or Allowed
func addAddressToAllowed(
	allowListConfig allowlist.AllowListConfig,
	addressStr string,
) allowlist.AllowListConfig {
	address := common.HexToAddress(addressStr)
	allowed := false
	if utils.Belongs(
		allowListConfig.AdminAddresses,
		address,
	) {
		allowed = true
	}
	if utils.Belongs(
		allowListConfig.ManagerAddresses,
		address,
	) {
		allowed = true
	}
	if utils.Belongs(
		allowListConfig.EnabledAddresses,
		address,
	) {
		allowed = true
	}
	if !allowed {
		allowListConfig.EnabledAddresses = append(
			allowListConfig.EnabledAddresses,
			address,
		)
	}
	return allowListConfig
}

func getPrecompiles(
	config params.ChainConfig,
	app *application.Avalanche,
	genesisTimestamp *uint64,
	useDefaults bool,
	useWarp bool,
	subnetEvmVersion string,
) (
	params.ChainConfig,
	statemachine.StateDirection,
	error,
) {
	if useDefaults || useWarp {
		warpConfig := configureWarp(genesisTimestamp)
		config.GenesisPrecompiles[warp.ConfigKey] = &warpConfig
	}

	if useDefaults {
		return config, statemachine.Forward, nil
	}

	const cancel = "Cancel"

	first := true

	remainingPrecompiles := []string{
		Warp,
		NativeMint,
		ContractAllowList,
		TxAllowList,
		FeeManager,
		RewardManager,
		cancel,
	}
	if useWarp {
		remainingPrecompiles = []string{
			NativeMint,
			ContractAllowList,
			TxAllowList,
			FeeManager,
			RewardManager,
			cancel,
		}
	}

	for {
		firstStr := "Advanced: Would you like to add a custom precompile to modify the EVM?"
		secondStr := "Would you like to add additional precompiles?"

		var promptStr string
		if promptStr = secondStr; first {
			promptStr = firstStr
			first = false
		}

		addPrecompile, err := app.Prompt.CaptureList(
			promptStr,
			[]string{prompts.No, prompts.Yes, goBackMsg},
		)
		if err != nil {
			return config, statemachine.Stop, err
		}

		switch addPrecompile {
		case prompts.No:
			return config, statemachine.Forward, nil
		case goBackMsg:
			return config, statemachine.Backward, nil
		}

		precompileDecision, err := app.Prompt.CaptureListWithSize(
			"Choose precompile",
			remainingPrecompiles,
			len(remainingPrecompiles),
		)
		if err != nil {
			return config, statemachine.Stop, err
		}

		switch precompileDecision {
		case NativeMint:
			mintConfig, cancelled, err := configureMinterList(app, subnetEvmVersion)
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
			contractConfig, cancelled, err := configureContractAllowList(app, subnetEvmVersion)
			if err != nil {
				return config, statemachine.Stop, err
			}
			if !cancelled {
				config.GenesisPrecompiles[deployerallowlist.ConfigKey] = &contractConfig
				remainingPrecompiles, err = removePrecompile(
					remainingPrecompiles,
					ContractAllowList,
				)
				if err != nil {
					return config, statemachine.Stop, err
				}
			}
		case TxAllowList:
			txConfig, cancelled, err := configureTransactionAllowList(app, subnetEvmVersion)
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
			feeConfig, cancelled, err := configureFeeConfigAllowList(app, subnetEvmVersion)
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
			rewardManagerConfig, cancelled, err := configureRewardManager(app, subnetEvmVersion)
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
		case Warp:
			warpConfig := configureWarp(genesisTimestamp)
			config.GenesisPrecompiles[warp.ConfigKey] = &warpConfig
			remainingPrecompiles, err = removePrecompile(remainingPrecompiles, Warp)
			if err != nil {
				return config, statemachine.Stop, err
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

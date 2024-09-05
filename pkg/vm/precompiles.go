// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanche-cli/pkg/utils"
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

func configureContractDeployerAllowList(
	params SubnetEVMGenesisParams,
) deployerallowlist.Config {
	config := deployerallowlist.Config{}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   params.contractDeployerPrecompileAllowList.AdminAddresses,
		ManagerAddresses: params.contractDeployerPrecompileAllowList.ManagerAddresses,
		EnabledAddresses: params.contractDeployerPrecompileAllowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config
}

func configureTransactionAllowList(
	params SubnetEVMGenesisParams,
) txallowlist.Config {
	config := txallowlist.Config{}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   params.transactionPrecompileAllowList.AdminAddresses,
		ManagerAddresses: params.transactionPrecompileAllowList.ManagerAddresses,
		EnabledAddresses: params.transactionPrecompileAllowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config
}

func configureNativeMinter(
	params SubnetEVMGenesisParams,
) nativeminter.Config {
	config := nativeminter.Config{}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   params.nativeMinterPrecompileAllowList.AdminAddresses,
		ManagerAddresses: params.nativeMinterPrecompileAllowList.ManagerAddresses,
		EnabledAddresses: params.nativeMinterPrecompileAllowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config
}

func configureFeeManager(
	params SubnetEVMGenesisParams,
) feemanager.Config {
	config := feemanager.Config{}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   params.feeManagerPrecompileAllowList.AdminAddresses,
		ManagerAddresses: params.feeManagerPrecompileAllowList.ManagerAddresses,
		EnabledAddresses: params.feeManagerPrecompileAllowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config
}

func configureRewardManager(
	params SubnetEVMGenesisParams,
) rewardmanager.Config {
	config := rewardmanager.Config{}
	config.AllowListConfig = allowlist.AllowListConfig{
		AdminAddresses:   params.rewardManagerPrecompileAllowList.AdminAddresses,
		ManagerAddresses: params.rewardManagerPrecompileAllowList.ManagerAddresses,
		EnabledAddresses: params.rewardManagerPrecompileAllowList.EnabledAddresses,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: subnetevmutils.NewUint64(0),
	}
	return config
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

// adds teleporter-related addresses (main funded key, messenger deploy key, relayer key)
// to the allow list of relevant enabled precompiles
//func addTeleporterAddressesToAllowLists(
//	config *params.ChainConfig,
//	teleporterAddress string,
//	teleporterMessengerDeployerAddress string,
//	relayerAddress string,
//) {
//	// tx allow list:
//	// teleporterAddress funds the other two and also deploys the registry
//	// teleporterMessengerDeployerAddress deploys the messenger
//	// relayerAddress is used by the relayer to send txs to the target chain
//	precompileConfig := config.GenesisPrecompiles[txallowlist.ConfigKey]
//	if precompileConfig != nil {
//		txAllowListConfig := precompileConfig.(*txallowlist.Config)
//		for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress, relayerAddress} {
//			txAllowListConfig.AllowListConfig = addAddressToAllowed(
//				txAllowListConfig.AllowListConfig,
//				address,
//			)
//		}
//	}
//	// contract deploy allow list:
//	// teleporterAddress deploys the registry
//	// teleporterMessengerDeployerAddress deploys the messenger
//	precompileConfig = config.GenesisPrecompiles[deployerallowlist.ConfigKey]
//	if precompileConfig != nil {
//		deployerAllowListConfig := precompileConfig.(*deployerallowlist.Config)
//		for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress} {
//			deployerAllowListConfig.AllowListConfig = addAddressToAllowed(
//				deployerAllowListConfig.AllowListConfig,
//				address,
//			)
//		}
//	}
//}

func addTeleporterAddressesToAllowLists(
	precompile *params.Precompiles,
	teleporterAddress string,
	teleporterMessengerDeployerAddress string,
	relayerAddress string,
) {
	// tx allow list:
	// teleporterAddress funds the other two and also deploys the registry
	// teleporterMessengerDeployerAddress deploys the messenger
	// relayerAddress is used by the relayer to send txs to the target chain
	currentPrecompile := *precompile
	precompileConfig := currentPrecompile[txallowlist.ConfigKey]
	if precompileConfig != nil {
		txAllowListConfig := precompileConfig.(*txallowlist.Config)
		for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress, relayerAddress} {
			txAllowListConfig.AllowListConfig = addAddressToAllowed(
				txAllowListConfig.AllowListConfig,
				address,
			)
		}
	}
	// contract deploy allow list:
	// teleporterAddress deploys the registry
	// teleporterMessengerDeployerAddress deploys the messenger
	precompileConfig = currentPrecompile[deployerallowlist.ConfigKey]
	if precompileConfig != nil {
		deployerAllowListConfig := precompileConfig.(*deployerallowlist.Config)
		for _, address := range []string{teleporterAddress, teleporterMessengerDeployerAddress} {
			deployerAllowListConfig.AllowListConfig = addAddressToAllowed(
				deployerAllowListConfig.AllowListConfig,
				address,
			)
		}
	}
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

//func getPrecompiles(
//	config *params.ChainConfig,
//	params SubnetEVMGenesisParams,
//	genesisTimestamp *uint64,
//) {
//	if params.enableWarpPrecompile {
//		warpConfig := configureWarp(genesisTimestamp)
//		config.GenesisPrecompiles[warp.ConfigKey] = &warpConfig
//	}
//
//	if params.enableNativeMinterPrecompile {
//		mintConfig := configureNativeMinter(params)
//		config.GenesisPrecompiles[nativeminter.ConfigKey] = &mintConfig
//	}
//
//	if params.enableContractDeployerPrecompile {
//		contractConfig := configureContractDeployerAllowList(params)
//		config.GenesisPrecompiles[deployerallowlist.ConfigKey] = &contractConfig
//	}
//	if params.enableTransactionPrecompile {
//		txConfig := configureTransactionAllowList(params)
//		config.GenesisPrecompiles[txallowlist.ConfigKey] = &txConfig
//	}
//	if params.enableFeeManagerPrecompile {
//		feeConfig := configureFeeManager(params)
//		config.GenesisPrecompiles[feemanager.ConfigKey] = &feeConfig
//	}
//	if params.enableRewardManagerPrecompile {
//		rewardManagerConfig := configureRewardManager(params)
//		config.GenesisPrecompiles[rewardmanager.ConfigKey] = &rewardManagerConfig
//	}
//}

func getPrecompiles(
	subnetEVMGenesisParams SubnetEVMGenesisParams,
	genesisTimestamp *uint64,
) params.Precompiles {
	precompiles := make(params.Precompiles)
	if subnetEVMGenesisParams.enableWarpPrecompile {
		warpConfig := configureWarp(genesisTimestamp)
		precompiles[warp.ConfigKey] = &warpConfig
	}

	if subnetEVMGenesisParams.enableNativeMinterPrecompile {
		mintConfig := configureNativeMinter(subnetEVMGenesisParams)
		precompiles[nativeminter.ConfigKey] = &mintConfig
	}

	if subnetEVMGenesisParams.enableContractDeployerPrecompile {
		contractConfig := configureContractDeployerAllowList(subnetEVMGenesisParams)
		precompiles[deployerallowlist.ConfigKey] = &contractConfig
	}
	if subnetEVMGenesisParams.enableTransactionPrecompile {
		txConfig := configureTransactionAllowList(subnetEVMGenesisParams)
		precompiles[txallowlist.ConfigKey] = &txConfig
	}
	if subnetEVMGenesisParams.enableFeeManagerPrecompile {
		feeConfig := configureFeeManager(subnetEVMGenesisParams)
		precompiles[feemanager.ConfigKey] = &feeConfig
	}
	if subnetEVMGenesisParams.enableRewardManagerPrecompile {
		rewardManagerConfig := configureRewardManager(subnetEVMGenesisParams)
		precompiles[rewardmanager.ConfigKey] = &rewardManagerConfig
	}
	return precompiles
}

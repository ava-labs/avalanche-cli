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
	"github.com/ethereum/go-ethereum/common"
)

func configureContractDeployerAllowList(
	params SubnetEVMGenesisParams,
	timestamp *uint64,
) deployerallowlist.Config {
	return deployerallowlist.Config{
		AllowListConfig: allowlist.AllowListConfig{
			AdminAddresses:   params.contractDeployerPrecompileAllowList.AdminAddresses,
			ManagerAddresses: params.contractDeployerPrecompileAllowList.ManagerAddresses,
			EnabledAddresses: params.contractDeployerPrecompileAllowList.EnabledAddresses,
		},
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

func configureTransactionAllowList(
	params SubnetEVMGenesisParams,
	timestamp *uint64,
) txallowlist.Config {
	return txallowlist.Config{
		AllowListConfig: allowlist.AllowListConfig{
			AdminAddresses:   params.transactionPrecompileAllowList.AdminAddresses,
			ManagerAddresses: params.transactionPrecompileAllowList.ManagerAddresses,
			EnabledAddresses: params.transactionPrecompileAllowList.EnabledAddresses,
		},
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

func configureNativeMinter(
	params SubnetEVMGenesisParams,
	timestamp *uint64,
) nativeminter.Config {
	return nativeminter.Config{
		AllowListConfig: allowlist.AllowListConfig{
			AdminAddresses:   params.nativeMinterPrecompileAllowList.AdminAddresses,
			ManagerAddresses: params.nativeMinterPrecompileAllowList.ManagerAddresses,
			EnabledAddresses: params.nativeMinterPrecompileAllowList.EnabledAddresses,
		},
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

func configureFeeManager(
	params SubnetEVMGenesisParams,
	timestamp *uint64,
) feemanager.Config {
	return feemanager.Config{
		AllowListConfig: allowlist.AllowListConfig{
			AdminAddresses:   params.feeManagerPrecompileAllowList.AdminAddresses,
			ManagerAddresses: params.feeManagerPrecompileAllowList.ManagerAddresses,
			EnabledAddresses: params.feeManagerPrecompileAllowList.EnabledAddresses,
		},
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

func configureRewardManager(
	params SubnetEVMGenesisParams,
	timestamp *uint64,
) rewardmanager.Config {
	return rewardmanager.Config{
		AllowListConfig: allowlist.AllowListConfig{
			AdminAddresses:   params.rewardManagerPrecompileAllowList.AdminAddresses,
			ManagerAddresses: params.rewardManagerPrecompileAllowList.ManagerAddresses,
			EnabledAddresses: params.rewardManagerPrecompileAllowList.EnabledAddresses,
		},
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

func configureWarp(timestamp *uint64) warp.Config {
	return warp.Config{
		QuorumNumerator: warp.WarpDefaultQuorumNumerator,
		Upgrade: precompileconfig.Upgrade{
			BlockTimestamp: timestamp,
		},
	}
}

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
		mintConfig := configureNativeMinter(subnetEVMGenesisParams, genesisTimestamp)
		precompiles[nativeminter.ConfigKey] = &mintConfig
	}

	if subnetEVMGenesisParams.enableContractDeployerPrecompile {
		contractConfig := configureContractDeployerAllowList(subnetEVMGenesisParams, genesisTimestamp)
		precompiles[deployerallowlist.ConfigKey] = &contractConfig
	}
	if subnetEVMGenesisParams.enableTransactionPrecompile {
		txConfig := configureTransactionAllowList(subnetEVMGenesisParams, genesisTimestamp)
		precompiles[txallowlist.ConfigKey] = &txConfig
	}
	if subnetEVMGenesisParams.enableFeeManagerPrecompile {
		feeConfig := configureFeeManager(subnetEVMGenesisParams, genesisTimestamp)
		precompiles[feemanager.ConfigKey] = &feeConfig
	}
	if subnetEVMGenesisParams.enableRewardManagerPrecompile {
		rewardManagerConfig := configureRewardManager(subnetEVMGenesisParams, genesisTimestamp)
		precompiles[rewardmanager.ConfigKey] = &rewardManagerConfig
	}
	return precompiles
}

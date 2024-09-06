// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanche-cli/pkg/tooling-sdk/utils"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	"github.com/ethereum/go-ethereum/common"
)

func ConfigureWarp(timestamp *uint64) warp.Config {
	config := warp.Config{
		QuorumNumerator: warp.WarpDefaultQuorumNumerator,
	}
	config.Upgrade = precompileconfig.Upgrade{
		BlockTimestamp: timestamp,
	}
	return config
}

// AddTeleporterAddressesToAllowLists adds teleporter-related addresses (main funded key, messenger
// deploy key, relayer key) to the allow list of relevant enabled precompiles
func AddTeleporterAddressesToAllowLists(
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

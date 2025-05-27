// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

const (
	baseDir      = constants.E2EBaseDirName
	hardhatDir   = "./tests/e2e/hardhat"
	confFilePath = hardhatDir + "/dynamic_conf.json"
	greeterFile  = hardhatDir + "/greeter.json"

	BaseTest               = "./test/index.ts"
	GreeterScript          = "./scripts/deploy.ts"
	GreeterCheck           = "./scripts/checkGreeting.ts"
	SoloSubnetEVMKey1      = "soloSubnetEVMVersion1"
	SoloSubnetEVMKey2      = "soloSubnetEVMVersion2"
	SoloAvagoKey           = "soloAvagoVersion"
	OnlyAvagoKey           = "onlyAvagoVersion"
	MultiAvagoSubnetEVMKey = "multiAvagoSubnetEVMVersion"
	MultiAvago1Key         = "multiAvagoVersion1"
	MultiAvago2Key         = "multiAvagoVersion2"
	LatestEVM2AvagoKey     = "latestEVM2Avago"
	LatestAvago2EVMKey     = "latestAvago2EVM"
	OnlyAvagoValue         = "latest"

	SubnetEvmGenesisPoaPath   = "tests/e2e/assets/test_subnet_evm_poa_genesis.json"
	SubnetEvmGenesisPoa2Path  = "tests/e2e/assets/test_subnet_evm_poa_genesis_2.json"
	SubnetEvmGenesisPath      = "tests/e2e/assets/test_subnet_evm_genesis.json"
	SubnetEvmGenesis2Path     = "tests/e2e/assets/test_subnet_evm_genesis_2.json"
	EwoqKeyPath               = "tests/e2e/assets/ewoq_key.pk"
	SubnetEvmAllowFeeRecpPath = "tests/e2e/assets/test_subnet_evm_allowFeeRecps_genesis.json"
	SubnetEvmGenesisBadPath   = "tests/e2e/assets/test_subnet_evm_genesis_bad.json"
	BootstrapValidatorPath    = "tests/e2e/assets/test_bootstrap_validator.json"
	BootstrapValidatorPath2   = "tests/e2e/assets/test_bootstrap_validator2.json"
	PluginDirExt              = "plugins"

	ledgerSimDir          = "./tests/e2e/ledgerSim"
	basicLedgerSimScript  = "./launchAndApproveTxs.ts"
	SubnetIDParseType     = "SubnetID"
	BlockchainIDParseType = "BlockchainID"

	BlockchainName = "testBlockchain"

	EwoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
)

var TestLocalNodeName = localnet.LocalClusterName(models.NewLocalNetwork(), BlockchainName)

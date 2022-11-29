// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

const (
	baseDir       = ".avalanche-cli"
	hardhatDir    = "./tests/e2e/hardhat"
	confFilePath  = hardhatDir + "/dynamic_conf.json"
	BaseTest      = "./test/index.ts"
	GreeterScript = "./scripts/deploy.ts"
	GreeterCheck  = "./scripts/checkGreeting.ts"
	greeterFile   = hardhatDir + "/greeter.json"

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

	SubnetEvmGenesisPath  = "tests/e2e/assets/test_subnet_evm_genesis.json"
	SubnetEvmGenesis2Path = "tests/e2e/assets/test_subnet_evm_genesis_2.json"
	SpacesVMGenesisPath   = "tests/e2e/assets/test_spacesvm_genesis.json"
	EwoqKeyPath           = "tests/e2e/assets/ewoq_key.pk"
	// FIXME
	SpacesVMVersion           = "v0.0.9"
	SubnetEvmAllowFeeRecpPath = "tests/e2e/assets/test_subnet_evm_allowFeeRecps_genesis.json"
)

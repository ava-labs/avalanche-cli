// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package models

import "github.com/ava-labs/avalanchego/utils/units"

type StaticFeeConfig struct {
	TxFee                         uint64
	CreateAssetTxFee              uint64
	CreateSubnetTxFee             uint64
	TransformSubnetTxFee          uint64
	CreateBlockchainTxFee         uint64
	AddPrimaryNetworkValidatorFee uint64
	AddPrimaryNetworkDelegatorFee uint64
	AddSubnetValidatorFee         uint64
	AddSubnetDelegatorFee         uint64
}

type StaticFeeParams struct {
	StaticConfig StaticFeeConfig
}

var (
	MainnetParams = StaticFeeParams{
		StaticConfig: StaticFeeConfig{
			TxFee:                         units.MilliAvax,
			CreateAssetTxFee:              10 * units.MilliAvax,
			CreateSubnetTxFee:             1 * units.Avax,
			TransformSubnetTxFee:          10 * units.Avax,
			CreateBlockchainTxFee:         1 * units.Avax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSubnetValidatorFee:         units.MilliAvax,
			AddSubnetDelegatorFee:         units.MilliAvax,
		},
	}
	FujiParams = StaticFeeParams{
		StaticConfig: StaticFeeConfig{
			TxFee:                         units.MilliAvax,
			CreateAssetTxFee:              10 * units.MilliAvax,
			CreateSubnetTxFee:             100 * units.MilliAvax,
			TransformSubnetTxFee:          1 * units.Avax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSubnetValidatorFee:         units.MilliAvax,
			AddSubnetDelegatorFee:         units.MilliAvax,
		},
	}
	LocalParams = StaticFeeParams{
		StaticConfig: StaticFeeConfig{
			TxFee:                         units.MilliAvax,
			CreateAssetTxFee:              units.MilliAvax,
			CreateSubnetTxFee:             100 * units.MilliAvax,
			TransformSubnetTxFee:          100 * units.MilliAvax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSubnetValidatorFee:         units.MilliAvax,
			AddSubnetDelegatorFee:         units.MilliAvax,
		},
	}
)

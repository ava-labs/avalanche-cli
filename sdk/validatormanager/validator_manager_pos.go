// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID] using PoS specific settings
func PoSValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID [32]byte,
	posParams PoSParams,
) (*types.Transaction, *types.Receipt, error) {
	if err := posParams.Verify(); err != nil {
		return nil, nil, err
	}
	var (
		defaultChurnPeriodSeconds     = uint64(0) // no churn period
		defaultMaximumChurnPercentage = uint8(20) // 20% of the validator set can be churned per churn period
	)

	type ValidatorManagerSettings struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}

	type NativeTokenValidatorManagerSettings struct {
		BaseSettings             ValidatorManagerSettings
		MinimumStakeAmount       *big.Int
		MaximumStakeAmount       *big.Int
		MinimumStakeDuration     uint64
		MinimumDelegationFeeBips uint16
		MaximumStakeMultiplier   uint8
		WeightToValueFactor      *big.Int
		RewardCalculator         common.Address
	}

	baseSettings := ValidatorManagerSettings{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
		MaximumChurnPercentage: defaultMaximumChurnPercentage,
	}

	params := NativeTokenValidatorManagerSettings{
		BaseSettings:             baseSettings,
		MinimumStakeAmount:       posParams.MinimumStakeAmount,
		MaximumStakeAmount:       posParams.MaximumStakeAmount,
		MinimumStakeDuration:     posParams.MinimumStakeDuration,
		MinimumDelegationFeeBips: posParams.MinimumDelegationFee,
		MaximumStakeMultiplier:   posParams.MaximumStakeMultiplier,
		WeightToValueFactor:      posParams.WeightToValueFactor,
		RewardCalculator:         common.HexToAddress(posParams.RewardCalculatorAddress),
	}

	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize Native Token PoS manager",
		ErrorSignatureToError,
		"initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address))",
		params,
	)
}

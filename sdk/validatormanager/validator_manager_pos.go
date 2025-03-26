// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
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
		UptimeBlockchainID:       posParams.UptimeBlockchainID,
	}

	return contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		managerAddress,
		nil,
		"initialize Native Token PoS manager",
		ErrorSignatureToError,
		"initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address,bytes32))",
		params,
	)
}

func PoSWeightToValue(
	rpcURL string,
	managerAddress common.Address,
	weight uint64,
) (*big.Int, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"weightToValue(uint64)->(uint256)",
		weight,
	)
	if err != nil {
		return nil, err
	}
	value, b := out[0].(*big.Int)
	if !b {
		return nil, fmt.Errorf("error at weightToValue, expected *big.Int, got %T", out[0])
	}
	return value, nil
}

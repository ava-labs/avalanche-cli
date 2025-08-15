// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanche-cli/sdk/evm/contract"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID] using PoS specific settings
func PoSValidatorManagerInitialize(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	specializedManagerAddress common.Address,
	managerOwnerPrivateKey string,
	privateKey string,
	subnetID [32]byte,
	posParams PoSParams,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if err := posParams.Verify(); err != nil {
		return nil, nil, err
	}
	const (
		defaultChurnPeriodSeconds     = uint64(0) // no churn period
		defaultMaximumChurnPercentage = uint8(20) // 20% of the validator set can be churned per churn period
	)
	if useACP99 {
		if tx, receipt, err := contract.TxToMethod(
			logger,
			rpcURL,
			false,
			common.Address{},
			privateKey,
			specializedManagerAddress,
			nil,
			"initialize Native Token PoS manager",
			ErrorSignatureToError,
			"initialize((address,uint256,uint256,uint64,uint16,uint8,uint256,address,bytes32))",
			NativeTokenValidatorManagerSettingsV2_0_0{
				Manager:                  managerAddress,
				MinimumStakeAmount:       posParams.MinimumStakeAmount,
				MaximumStakeAmount:       posParams.MaximumStakeAmount,
				MinimumStakeDuration:     posParams.MinimumStakeDuration,
				MinimumDelegationFeeBips: posParams.MinimumDelegationFee,
				MaximumStakeMultiplier:   posParams.MaximumStakeMultiplier,
				WeightToValueFactor:      posParams.WeightToValueFactor,
				RewardCalculator:         common.HexToAddress(posParams.RewardCalculatorAddress),
				UptimeBlockchainID:       posParams.UptimeBlockchainID,
			},
		); err != nil {
			return tx, receipt, err
		}
		managerOwnerAddress, err := evm.PrivateKeyToAddress(managerOwnerPrivateKey)
		if err != nil {
			return nil, nil, err
		}
		client, err := evm.GetClient(rpcURL)
		if err != nil {
			return nil, nil, err
		}
		_, err = client.FundAddress(
			privateKey,
			managerOwnerAddress.Hex(),
			big.NewInt(100_000_000_000_000_000), // 0.1 TOKEN
		)
		if err != nil {
			return nil, nil, err
		}
		err = contract.TransferOwnership(
			logger,
			rpcURL,
			managerAddress,
			managerOwnerPrivateKey,
			specializedManagerAddress,
		)
		return nil, nil, err
	}
	return contract.TxToMethod(
		logger,
		rpcURL,
		false,
		common.Address{},
		privateKey,
		managerAddress,
		nil,
		"initialize Native Token PoS manager",
		ErrorSignatureToError,
		"initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address,bytes32))",
		NativeTokenValidatorManagerSettingsV1_0_0{
			BaseSettings: ValidatorManagerSettings{
				SubnetID:               subnetID,
				ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
				MaximumChurnPercentage: defaultMaximumChurnPercentage,
			},
			MinimumStakeAmount:       posParams.MinimumStakeAmount,
			MaximumStakeAmount:       posParams.MaximumStakeAmount,
			MinimumStakeDuration:     posParams.MinimumStakeDuration,
			MinimumDelegationFeeBips: posParams.MinimumDelegationFee,
			MaximumStakeMultiplier:   posParams.MaximumStakeMultiplier,
			WeightToValueFactor:      posParams.WeightToValueFactor,
			RewardCalculator:         common.HexToAddress(posParams.RewardCalculatorAddress),
			UptimeBlockchainID:       posParams.UptimeBlockchainID,
		},
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

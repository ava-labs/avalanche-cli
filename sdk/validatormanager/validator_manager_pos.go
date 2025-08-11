// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanche-cli/sdk/evm/precompiles"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID] using PoS specific settings
func PoSValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	specializedManagerAddress common.Address,
	managerOwnerPrivateKey string,
	privateKey string,
	subnetID [32]byte,
	posParams PoSParams,
	useACP99 bool,
	nativeMinterPrecompileAdminPrivateKey string,
) (*types.Transaction, *types.Receipt, error) {
	if err := posParams.Verify(); err != nil {
		return nil, nil, err
	}
	if useACP99 {
		client, err := evm.GetClient(rpcURL)
		if err != nil {
			return nil, nil, err
		}
		nativeMinterPrecompileOn, err := client.ContractAlreadyDeployed(precompiles.NativeMinterPrecompile.Hex())
		if err != nil {
			return nil, nil, err
		}
		if !nativeMinterPrecompileOn {
			return nil, nil, fmt.Errorf("native minter precompile should be enabled for Native PoS")
		}
		allowedStatus, err := precompiles.ReadAllowList(
			rpcURL,
			precompiles.NativeMinterPrecompile,
			specializedManagerAddress,
		)
		if err != nil {
			return nil, nil, err
		}
		if allowedStatus.Cmp(big.NewInt(0)) == 0 {
			if nativeMinterPrecompileAdminPrivateKey == "" {
				return nil, nil, fmt.Errorf("no managed native minter precompile admin was found, and need to be used to enable Native PoS")
			}
			if err := precompiles.SetEnabled(
				rpcURL,
				precompiles.NativeMinterPrecompile,
				nativeMinterPrecompileAdminPrivateKey,
				specializedManagerAddress,
			); err != nil {
				return nil, nil, err
			}
		}
		weightToValueFactor := new(big.Int)
		weightToValueFactor.Mul(posParams.WeightToValueFactor, big.NewInt(1_000_000_000_000_000_000))
		minimumStakeAmount := new(big.Int)
		minimumStakeAmount.Mul(posParams.MinimumStakeAmount, big.NewInt(1_000_000_000_000_000_000))
		maximumStakeAmount := new(big.Int)
		maximumStakeAmount.Mul(posParams.MaximumStakeAmount, big.NewInt(1_000_000_000_000_000_000))
		if tx, receipt, err := contract.TxToMethod(
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
				MinimumStakeAmount:       minimumStakeAmount,
				MaximumStakeAmount:       maximumStakeAmount,
				MinimumStakeDuration:     posParams.MinimumStakeDuration,
				MinimumDelegationFeeBips: posParams.MinimumDelegationFee,
				MaximumStakeMultiplier:   posParams.MaximumStakeMultiplier,
				WeightToValueFactor:      weightToValueFactor,
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
		_, err = client.FundAddress(
			privateKey,
			managerOwnerAddress.Hex(),
			big.NewInt(100_000_000_000_000_000), // 0.1 TOKEN
		)
		if err != nil {
			return nil, nil, err
		}
		err = contract.TransferOwnership(
			rpcURL,
			managerAddress,
			managerOwnerPrivateKey,
			specializedManagerAddress,
		)
		return nil, nil, err
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
		NativeTokenValidatorManagerSettingsV1_0_0{
			BaseSettings: ValidatorManagerSettings{
				SubnetID:               subnetID,
				ChurnPeriodSeconds:     ChurnPeriodSeconds,
				MaximumChurnPercentage: MaximumChurnPercentage,
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
		nil,
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

type GetStakingManagerSetttingsReturn struct {
	ValidatorManager         common.Address
	MinimumStakeAmount       *big.Int
	MaximumStakeAmount       *big.Int
	MinimumStakeDuration     uint64
	MinimumDelegationFeeBips uint16
	MaximumStakeMultiplier   uint8
	WeightToValueFactor      *big.Int
	RewardCalculator         common.Address
	UptimeBlockchainID       [32]byte
}

func GetStakingManagerSettings(
	rpcURL string,
	managerAddress common.Address,
) (GetStakingManagerSetttingsReturn, error) {
	getStakingManagerSetttingsReturn := GetStakingManagerSetttingsReturn{}
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"getStakingManagerSettings()->(address,uint256,uint256,uint64,uint16,uint8,uint256,address,bytes32)",
		nil,
	)
	if err != nil {
		return getStakingManagerSetttingsReturn, err
	}
	if len(out) != 9 {
		return getStakingManagerSetttingsReturn, fmt.Errorf("incorrect number of outputs for getStakingManagerSettings: expected 9 got %d", len(out))
	}
	var ok bool
	getStakingManagerSetttingsReturn.ValidatorManager, ok = out[0].(common.Address)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for validatorManager output of getStakingManagerSettings: expected common.Address, got %T", out[0])
	}
	getStakingManagerSetttingsReturn.MinimumStakeAmount, ok = out[1].(*big.Int)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for minimumStakeAmount output of getStakingManagerSettings: expected *big.Int, got %T", out[1])
	}
	getStakingManagerSetttingsReturn.MaximumStakeAmount, ok = out[2].(*big.Int)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for maximumStakeAmount output of getStakingManagerSettings: expected *big.Int, got %T", out[2])
	}
	getStakingManagerSetttingsReturn.MinimumStakeDuration, ok = out[3].(uint64)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for minimumStakeDuration output of getStakingManagerSettings: expected uint64, got %T", out[3])
	}
	getStakingManagerSetttingsReturn.MinimumDelegationFeeBips, ok = out[4].(uint16)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for minimumDelegationFeeBips output of getStakingManagerSettings: expected uint16, got %T", out[4])
	}
	getStakingManagerSetttingsReturn.MaximumStakeMultiplier, ok = out[5].(uint8)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for maximumStakeMultiplier output of getStakingManagerSettings: expected uint8, got %T", out[5])
	}
	getStakingManagerSetttingsReturn.WeightToValueFactor, ok = out[6].(*big.Int)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for weightToValueFactor output of getStakingManagerSettings: expected *big.Int, got %T", out[6])
	}
	getStakingManagerSetttingsReturn.RewardCalculator, ok = out[7].(common.Address)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for rewardCalculator output of getStakingManagerSettings: expected common.Address, got %T", out[7])
	}
	getStakingManagerSetttingsReturn.UptimeBlockchainID, ok = out[8].([32]byte)
	if !ok {
		return getStakingManagerSetttingsReturn, fmt.Errorf("invalid type for uptimeBlockchainID output of getStakingManagerSettings: expected [32]byte, got %T", out[8])
	}
	return getStakingManagerSetttingsReturn, nil
}

type GetStakingValidatorReturn struct {
	Owner             common.Address
	DelegationFeeBips uint16
	MinStakeDuration  uint64
	UptimeSeconds     uint64
}

func GetStakingValidator(
	rpcURL string,
	managerAddress common.Address,
	validationID ids.ID,
) (GetStakingValidatorReturn, error) {
	getStakingValidatorReturn := GetStakingValidatorReturn{}
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"getStakingValidator(bytes32)->(address,uint16,uint64,uint64)",
		nil,
		[32]byte(validationID),
	)
	if err != nil {
		return getStakingValidatorReturn, err
	}
	if len(out) != 4 {
		return getStakingValidatorReturn, fmt.Errorf("incorrect number of outputs for getStakingValidator: expected 4 got %d", len(out))
	}
	var ok bool
	getStakingValidatorReturn.Owner, ok = out[0].(common.Address)
	if !ok {
		return getStakingValidatorReturn, fmt.Errorf("invalid type for owner output of getStakingValidator: expected common.Address, got %T", out[0])
	}
	getStakingValidatorReturn.DelegationFeeBips, ok = out[1].(uint16)
	if !ok {
		return getStakingValidatorReturn, fmt.Errorf("invalid type for delegationFeeBips output of getStakingValidator: expected uint16, got %T", out[1])
	}
	getStakingValidatorReturn.MinStakeDuration, ok = out[2].(uint64)
	if !ok {
		return getStakingValidatorReturn, fmt.Errorf("invalid type for minStakeDuration output of getStakingValidator: expected uint64, got %T", out[2])
	}
	getStakingValidatorReturn.UptimeSeconds, ok = out[3].(uint64)
	if !ok {
		return getStakingValidatorReturn, fmt.Errorf("invalid type for uptimeSeconds output of getStakingValidator: expected uint64, got %T", out[3])
	}
	return getStakingValidatorReturn, nil
}

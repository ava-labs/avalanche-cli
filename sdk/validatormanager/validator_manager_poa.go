// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

// PoAValidatorManagerInitialize initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID], with
// owner given by [ownerAddress]
func PoAValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if useACP99 {
		return contract.TxToMethod(
			rpcURL,
			false,
			common.Address{},
			privateKey,
			managerAddress,
			nil,
			"initialize PoA manager",
			ErrorSignatureToError,
			"initialize((address, bytes32,uint64,uint8))",
			ACP99ValidatorManagerSettings{
				Admin:                  ownerAddress,
				SubnetID:               subnetID,
				ChurnPeriodSeconds:     ChurnPeriodSeconds,
				MaximumChurnPercentage: MaximumChurnPercentage,
			},
		)
	}
	return contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		managerAddress,
		nil,
		"initialize PoA manager",
		ErrorSignatureToError,
		"initialize((bytes32,uint64,uint8),address)",
		ValidatorManagerSettings{
			SubnetID:               subnetID,
			ChurnPeriodSeconds:     ChurnPeriodSeconds,
			MaximumChurnPercentage: MaximumChurnPercentage,
		},
		ownerAddress,
	)
}

type GetValidatorReturn struct {
	Status         uint8
	NodeID         []byte
	StartingWeight uint64
	SentNonce      uint64
	ReceivedNonce  uint64
	Weight         uint64
	StartTime      uint64
	EndTime        uint64
}

func GetValidator(
	rpcURL string,
	managerAddress common.Address,
	validationID ids.ID,
) (*GetValidatorReturn, error) {
	stakingManagerSettings, err := GetStakingManagerSettings(
		rpcURL,
		managerAddress,
	)
	if err == nil {
		// fix address if specialized
		managerAddress = stakingManagerSettings.ValidatorManager
	}
	getValidatorReturn := &GetValidatorReturn{}
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"getValidator(bytes32)->((uint8,bytes,uint64,uint64,uint64,uint64,uint64,uint64))",
		[]interface{}{*getValidatorReturn},
		[32]byte(validationID),
	)
	if err != nil {
		return getValidatorReturn, err
	}
	if len(out) != 1 {
		return getValidatorReturn, fmt.Errorf("incorrect number of outputs for getValidator: expected 1 got %d", len(out))
	}
	var ok bool
	getValidatorReturn, ok = abi.ConvertType(out[0], new(GetValidatorReturn)).(*GetValidatorReturn)
	if !ok {
		return getValidatorReturn, fmt.Errorf("invalid type for output of getValidator: expected GetValidatorReturn, got %T", out[0])
	}
	return getValidatorReturn, nil
}

type ChurnSettings struct {
	ChurnPeriodSeconds     uint64
	MaximumChurnPercentage uint8
}

func GetChurnSettings(
	rpcURL string,
	managerAddress common.Address,
) (ChurnSettings, error) {
	stakingManagerSettings, err := GetStakingManagerSettings(
		rpcURL,
		managerAddress,
	)
	if err == nil {
		// fix address if specialized
		managerAddress = stakingManagerSettings.ValidatorManager
	}
	churnSettings := ChurnSettings{}
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"getChurnTracker()->(uint64,uint8,uint256,uint64,uint64,uint64)",
		nil,
	)
	if err != nil {
		return churnSettings, err
	}
	if len(out) != 6 {
		return churnSettings, fmt.Errorf("incorrect number of outputs for getChurnTracker: expected 6 got %d", len(out))
	}
	var ok bool
	churnSettings.ChurnPeriodSeconds, ok = out[0].(uint64)
	if !ok {
		return churnSettings, fmt.Errorf("invalid type for churnPeriodSeconds output of getChurnTracker: expected uint64, got %T", out[0])
	}
	churnSettings.MaximumChurnPercentage, ok = out[1].(uint8)
	if !ok {
		return churnSettings, fmt.Errorf("invalid type for maximumChurnPercentage output of getChurnTracker: expected uint8, got %T", out[1])
	}
	return churnSettings, nil
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"github.com/ava-labs/avalanche-cli/sdk/evm/contract"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/ids"
)

// PoAValidatorManagerInitialize initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID], with
// owner given by [ownerAddress]
func PoAValidatorManagerInitialize(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	const (
		defaultChurnPeriodSeconds     = uint64(0)
		defaultMaximumChurnPercentage = uint8(20)
	)
	if useACP99 {
		return contract.TxToMethod(
			logger,
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
				ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
				MaximumChurnPercentage: defaultMaximumChurnPercentage,
			},
		)
	}
	return contract.TxToMethod(
		logger,
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
			ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
			MaximumChurnPercentage: defaultMaximumChurnPercentage,
		},
		ownerAddress,
	)
}

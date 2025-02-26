// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package clierrors

import "errors"

var (
	ErrNoBlockchainID                 = errors.New("failed to find the blockchain ID for this subnet, has it been deployed/created on this network?")
	ErrNoSubnetID                     = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
	ErrInvalidValidatorManagerAddress = errors.New("invalid validator manager address")
)

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package clierrors

import "errors"

var (
	ErrNoBlockchainID                 = errors.New("failed to find the blockchain ID for this subnet, has it been deployed/created on this network?\nyou can use 'avalanche blockchain import' if having partial information on a deployed subnet")
	ErrNoSubnetID                     = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?\nyou can use 'avalanche blockchain import' if having partial information on a deployed subnet")
	ErrInvalidValidatorManagerAddress = errors.New("invalid validator manager address")
)

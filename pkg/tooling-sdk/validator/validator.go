// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validator

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

type PrimaryNetworkValidatorParams struct {
	// NodeID is the unique identifier of the node to be added as a validator on the Primary Network.
	NodeID ids.NodeID

	// Duration is how long the node will be staking the Primary Network
	// Duration has to be greater than or equal to minimum duration for the specified network
	// (Fuji / Mainnet)
	Duration time.Duration

	// StakeAmount is the amount of Avalanche tokens (AVAX) to stake in this validator, which is
	// denominated in nAVAX. StakeAmount has to be greater than or equal to minimum stake required
	// for the specified network
	StakeAmount uint64

	// DelegationFee is the percent fee this validator will charge when others delegate stake to it
	// When DelegationFee is not set, the minimum delegation fee for the specified network will be set
	// For more information on delegation fee, please head to https://docs.avax.network/nodes/validate/node-validator#delegation-fee-rate
	DelegationFee uint32
}

type SubnetValidatorParams struct {
	// NodeID is the unique identifier of the node to be added as a validator on the specified Subnet.
	NodeID ids.NodeID
	// Duration is how long the node will be staking the Subnet
	// Duration has to be less than or equal to the duration that the node will be validating the Primary
	// Network
	Duration time.Duration
	// Weight is the validator's weight when sampling validators.
	// Weight for subnet validators is set to 20 by default
	Weight uint64
}

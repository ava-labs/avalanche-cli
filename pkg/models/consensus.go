// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type ConsensusType string

const (
	ProofOfStake       = "Proof Of Stake"
	ProofOfAuthority   = "Proof Of Authority"
	UndefinedConsensus = "Undefined Consensus"
)

func ConsensusTypeFromString(s string) ConsensusType {
	switch s {
	case ProofOfStake:
		return ProofOfStake
	case ProofOfAuthority:
		return ProofOfAuthority
	default:
		return UndefinedConsensus
	}
}

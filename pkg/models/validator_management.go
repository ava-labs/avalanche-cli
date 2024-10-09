// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type ValidatorManagementType string

const (
	ProofOfStake                 = "Proof Of Stake (Coming Soon)"
	ProofOfAuthority             = "Proof Of Authority"
	UndefinedValidatorManagement = "Undefined Validator Management"
)

func ValidatorManagementTypeFromString(s string) ValidatorManagementType {
	switch s {
	case ProofOfStake:
		return ProofOfStake
	case ProofOfAuthority:
		return ProofOfAuthority
	default:
		return UndefinedValidatorManagement
	}
}

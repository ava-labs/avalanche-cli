// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanagertypes

type ValidatorManagementType string

const (
	ProofOfStake                 = "Proof Of Stake"
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

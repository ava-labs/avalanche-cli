// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "github.com/ava-labs/avalanche-cli/pkg/constants"

type VMType string

const (
	SubnetEvm   = "SubnetEVM"
	SpacesVM    = "SpacesVM"
	BlobVM      = "Blob VM"
	TimestampVM = "Timestamp VM"
	CustomVM    = "Custom"
)

func VMTypeFromString(s string) VMType {
	switch s {
	case SubnetEvm:
		return SubnetEvm
	case SpacesVM:
		return SpacesVM
	case BlobVM:
		return BlobVM
	case TimestampVM:
		return TimestampVM
	default:
		return CustomVM
	}
}

func (v VMType) RepoName() string {
	switch v {
	case SubnetEvm:
		return constants.SubnetEVMRepoName
	case SpacesVM:
		return constants.SpacesVMRepoName
	default:
		return "unknown"
	}
}

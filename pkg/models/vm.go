// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type VmType string

const (
	SubnetEvm   = "SubnetEVM"
	SpacesVm    = "Spaces VM"
	BlobVm      = "Blob VM"
	TimestampVm = "Timestamp VM"
	CustomVm    = "Custom"
)

func VmTypeFromString(s string) VmType {
	switch s {
	case SubnetEvm:
		return SubnetEvm
	case SpacesVm:
		return SpacesVm
	case BlobVm:
		return BlobVm
	case TimestampVm:
		return TimestampVm
	default:
		return CustomVm
	}
}

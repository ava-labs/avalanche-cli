// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
)

type NetworkData struct {
	SubnetID     ids.ID
	BlockchainID ids.ID
}

type Sidecar struct {
	Name            string
	VM              VMType
	VMVersion       string
	Subnet          string
	TokenName       string
	ChainID         string
	Version         string
	Networks        map[string]NetworkData
	ImportedFromAPM bool
	ImportedVMID    string
}

func (sc Sidecar) GetVMID() (string, error) {
	// get vmid
	var vmid string
	if sc.ImportedFromAPM {
		vmid = sc.ImportedVMID
	} else {
		chainVMID, err := utils.VMID(sc.Name)
		if err != nil {
			return "", err
		}
		vmid = chainVMID.String()
	}
	return vmid, nil
}

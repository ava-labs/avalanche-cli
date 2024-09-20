// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
)

type NetworkData struct {
	SubnetID                   ids.ID
	BlockchainID               ids.ID
	RPCVersion                 int
	TeleporterMessengerAddress string
	TeleporterRegistryAddress  string
	RPCEndpoints               []string
	WSEndpoints                []string
	BootstrapValidators        []SubnetValidator
}

type Sidecar struct {
	Name                string
	VM                  VMType
	VMVersion           string
	RPCVersion          int
	Subnet              string
	ExternalToken       bool
	TokenName           string
	TokenSymbol         string
	ChainID             string
	Version             string
	Networks            map[string]NetworkData
	ImportedFromAPM     bool
	ImportedVMID        string
	CustomVMRepoURL     string
	CustomVMBranch      string
	CustomVMBuildScript string
	// Teleporter related
	TeleporterReady   bool
	TeleporterKey     string
	TeleporterVersion string
	RunRelayer        bool
	// SubnetEVM based VM's only
	SubnetEVMMainnetChainID uint
	// TODO: remove if not needed for subnet acp 77 create flow once avalnache go releases etna
	ValidatorManagement        ValidatorManagementType
	ValidatorManagerController string
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

func (sc Sidecar) NetworkDataIsEmpty(network string) bool {
	if sc.Networks[network].SubnetID == ids.Empty &&
		sc.Networks[network].BlockchainID == ids.Empty &&
		sc.Networks[network].RPCVersion == 0 &&
		sc.Networks[network].TeleporterMessengerAddress == "" &&
		sc.Networks[network].TeleporterRegistryAddress == "" {
		return true
	}
	return false
}

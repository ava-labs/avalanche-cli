// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/ids"
)

type NetworkData struct {
	SubnetID                           ids.ID
	BlockchainID                       ids.ID
	RPCVersion                         int
	TeleporterMessengerAddress         string
	TeleporterRegistryAddress          string
	RPCEndpoints                       []string
	WSEndpoints                        []string
	BootstrapValidators                []SubnetValidator
	ClusterName                        string
	ValidatorManagerAddress            string
	ValidatorManagerBlockchainID       ids.ID
	ValidatorManagerRPCEndpoint        string
	SpecializedValidatorManagerAddress string
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
	// ICM related
	TeleporterReady   bool
	TeleporterKey     string
	TeleporterVersion string
	RunRelayer        bool
	// SubnetEVM based VM's only
	SubnetEVMMainnetChainID uint
	// TODO: remove if not needed for subnet acp 77 create flow once avalnache go releases etna
	ValidatorManagement   validatormanagertypes.ValidatorManagementType
	ValidatorManagerOwner string
	ProxyContractOwner    string
	// Subnet defaults to Sovereign post ACP-77
	Sovereign bool
	UseACP99  bool
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
	_, networkExists := sc.Networks[network]
	return !networkExists
}

func (sc Sidecar) PoA() bool {
	return sc.ValidatorManagement == validatormanagertypes.ProofOfAuthority
}

func (sc Sidecar) PoS() bool {
	return sc.ValidatorManagement == validatormanagertypes.ProofOfStake
}

func (sc Sidecar) UpdateValidatorManagerAddress(
	network string,
	managerRPCEndpoint string,
	managerBlockchainID ids.ID,
	managerAddr string,
	specializedManagerAddr string,
) {
	temp := sc.Networks[network]
	temp.ValidatorManagerRPCEndpoint = managerRPCEndpoint
	temp.ValidatorManagerBlockchainID = managerBlockchainID
	temp.ValidatorManagerAddress = managerAddr
	temp.SpecializedValidatorManagerAddress = specializedManagerAddr
	sc.Networks[network] = temp
}

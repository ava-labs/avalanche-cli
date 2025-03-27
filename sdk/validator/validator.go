// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validator

import (
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/maps"
)

type ValidatorKind int64

const (
	UndefinedValidatorKind ValidatorKind = iota
	NonValidator
	SovereignValidator
	NonSovereignValidator
)

func GetTotalWeight(net network.Network, subnetID ids.ID) (uint64, error) {
	pClient := platformvm.NewClient(net.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	validators, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
	if err != nil {
		return 0, err
	}
	weight := uint64(0)
	for _, vdr := range validators {
		weight += vdr.Weight
	}
	return weight, nil
}

func IsValidator(net network.Network, subnetID ids.ID, nodeID ids.NodeID) (bool, error) {
	pClient := platformvm.NewClient(net.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	validators, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
	if err != nil {
		return false, err
	}
	nodeIDs := maps.Keys(validators)
	return utils.Belongs(nodeIDs, nodeID), nil
}

func GetValidatorBalance(net network.Network, validationID ids.ID) (uint64, error) {
	vdrInfo, err := GetValidatorInfo(net, validationID)
	if err != nil {
		return 0, err
	}
	return vdrInfo.Balance, nil
}

func GetValidatorInfo(net network.Network, validationID ids.ID) (platformvm.L1Validator, error) {
	pClient := platformvm.NewClient(net.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	vdrInfo, _, err := pClient.GetL1Validator(ctx, validationID)
	if err != nil {
		return platformvm.L1Validator{}, err
	}
	return vdrInfo, nil
}

// Returns the validation ID for the Node ID, as registered at the validator manager
// Will return ids.Empty in case it is not registered
func GetValidationID(
	rpcURL string,
	managerAddress common.Address,
	nodeID ids.NodeID,
) (ids.ID, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"registeredValidators(bytes)->(bytes32)",
		nodeID[:],
	)
	if err != nil {
		return ids.Empty, err
	}
	return contract.GetSmartContractCallResult[[32]byte]("registeredValidators", out)
}

func IsSovereignValidator(
	network network.Network,
	subnetID ids.ID,
	nodeID ids.NodeID,
) (ValidatorKind, error) {
	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	vs, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return UndefinedValidatorKind, err
	}
	for _, v := range vs {
		if v.NodeID == nodeID {
			if v.TxID == ids.Empty {
				return SovereignValidator, nil
			}
			return NonSovereignValidator, nil
		}
	}
	return NonValidator, nil
}

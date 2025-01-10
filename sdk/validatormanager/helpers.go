// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/maps"
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

func GetValidationID(rpcURL string, nodeID ids.NodeID) (ids.ID, error) {
	managerAddress := common.HexToAddress(ProxyContractAddress)
	return GetRegisteredValidator(rpcURL, managerAddress, nodeID)
}

func GetRegisteredValidator(
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
	validatorID, b := out[0].([32]byte)
	if !b {
		return ids.Empty, fmt.Errorf("error at registeredValidators call, expected [32]byte, got %T", out[0])
	}
	return validatorID, nil
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validator

import (
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/sdk/evm/contract"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	avalanchegojson "github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/rpc"
	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/ethereum/go-ethereum/common"
)

type ValidatorKind int64

const (
	UndefinedValidatorKind ValidatorKind = iota
	NonValidator
	SovereignValidator
	NonSovereignValidator
)

// To enable querying validation IDs from P-Chain
type CurrentValidatorInfo struct {
	Weight       avalanchegojson.Uint64 `json:"weight"`
	NodeID       ids.NodeID             `json:"nodeID"`
	ValidationID ids.ID                 `json:"validationID"`
	Balance      avalanchegojson.Uint64 `json:"balance"`
}

func GetTotalWeight(network network.Network, subnetID ids.ID) (uint64, error) {
	validators, err := GetCurrentValidators(network, subnetID)
	if err != nil {
		return 0, err
	}
	weight := uint64(0)
	for _, vdr := range validators {
		weight += uint64(vdr.Weight)
	}
	return weight, nil
}

func IsValidator(network network.Network, subnetID ids.ID, nodeID ids.NodeID) (bool, error) {
	validators, err := GetCurrentValidators(network, subnetID)
	if err != nil {
		return false, err
	}
	nodeIDs := utils.Map(validators, func(v CurrentValidatorInfo) ids.NodeID { return v.NodeID })
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
	// if specialized, need to retrieve underlying manager
	// needs to directly access the manager, does not work with a proxy
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"getStakingManagerSettings()->(address,uint256,uint256,uint64,uint16,uint8,uint256,address,bytes32)",
	)
	if err == nil && len(out) == 9 {
		validatorManager, ok := out[0].(common.Address)
		if ok {
			managerAddress = validatorManager
		}
	}
	out, err = contract.CallToMethod(
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

func GetValidatorKind(
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

// Enables querying the validation IDs from P-Chain
func GetCurrentValidators(network network.Network, subnetID ids.ID) ([]CurrentValidatorInfo, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	requester := rpc.NewEndpointRequester(network.Endpoint + "/ext/P")
	res := &platformvm.GetCurrentValidatorsReply{}
	if err := requester.SendRequest(
		ctx,
		"platform.getCurrentValidators",
		&platformvm.GetCurrentValidatorsArgs{
			SubnetID: subnetID,
			NodeIDs:  nil,
		},
		res,
	); err != nil {
		return nil, err
	}
	validators := make([]CurrentValidatorInfo, 0, len(res.Validators))
	for _, vI := range res.Validators {
		vBytes, err := json.Marshal(vI)
		if err != nil {
			return nil, err
		}
		var v CurrentValidatorInfo
		if err := json.Unmarshal(vBytes, &v); err != nil {
			return nil, err
		}
		validators = append(validators, v)
	}
	return validators, nil
}

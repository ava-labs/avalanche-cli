// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIsNodeValidatingSubnet(t *testing.T) {
	require := require.New(t)
	nodeID := ids.GenerateTestNodeID()
	nonValidator := ids.GenerateTestNodeID()
	subnetID := ids.GenerateTestID()

	pClient := &mocks.PClient{}
	pClient.On("GetCurrentValidators", mock.Anything, mock.Anything, mock.Anything).Return(
		[]platformvm.ClientPermissionlessValidator{
			{
				ClientStaker: platformvm.ClientStaker{
					NodeID: nodeID,
				},
			},
		}, nil)

	pClient.On("GetPendingValidators", mock.Anything, mock.Anything, mock.Anything).Return(
		[]interface{}{}, nil, nil).Once()

	interfaceReturn := make([]interface{}, 1)
	val := map[string]interface{}{
		"nodeID": nonValidator.String(),
	}
	interfaceReturn[0] = val
	pClient.On("GetPendingValidators", mock.Anything, mock.Anything, mock.Anything).Return(interfaceReturn, nil, nil)

	// first pass: should return true for the GetCurrentValidators
	isValidating, err := checkIsValidating(subnetID, nodeID, pClient)
	require.NoError(err)
	require.True(isValidating)

	// second pass: The nonValidator is not in current nor pending validators, hence false
	isValidating, err = checkIsValidating(subnetID, nonValidator, pClient)
	require.NoError(err)
	require.False(isValidating)

	// third pass: The second mocked GetPendingValidators applies, and this time
	// nonValidator is in the pending set, hence true
	isValidating, err = checkIsValidating(subnetID, nonValidator, pClient)
	require.NoError(err)
	require.True(isValidating)
}

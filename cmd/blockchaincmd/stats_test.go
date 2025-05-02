// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"io"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	require := require.New(t)

	ux.NewUserLog(logging.NoLog{}, io.Discard)

	pClient := &mocks.PClient{}
	iClient := &mocks.InfoClient{}

	localNodeID := ids.GenerateTestNodeID()
	subnetID := ids.GenerateTestID()

	startTime := time.Now()
	endTime := time.Now()
	weight := uint64(42)
	conn := true

	remaining := ux.FormatDuration(endTime.Sub(startTime))

	reply := []platformvm.ClientPermissionlessValidator{
		{
			ClientStaker: platformvm.ClientStaker{
				StartTime: uint64(startTime.Unix()),
				EndTime:   uint64(endTime.Unix()),
				NodeID:    localNodeID,
				Weight:    weight,
			},
			Connected: &conn,
		},
	}

	pClient.On("GetCurrentValidators", mock.Anything, mock.Anything, mock.Anything).Return(reply, nil)
	iClient.On("GetNodeID", mock.Anything).Return(localNodeID, nil, nil)
	iClient.On("GetNodeVersion", mock.Anything).Return(&info.GetNodeVersionReply{
		VMVersions: map[string]string{
			subnetID.String(): "0.1.23",
		},
	}, nil)

	table := tablewriter.NewWriter(io.Discard)

	expectedVerStr := subnetID.String() + ": 0.1.23\n"

	rows, err := buildCurrentValidatorStats(pClient, iClient, table, subnetID)
	table.Append(rows[0])

	require.NoError(err)
	require.Equal(1, table.NumLines())
	require.Equal(localNodeID.String(), rows[0][0])
	require.Equal("true", rows[0][1])
	require.Equal("42", rows[0][2])
	require.Equal(remaining, rows[0][3])
	require.Equal(expectedVerStr, rows[0][4])
}

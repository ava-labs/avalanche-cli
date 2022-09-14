// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"io"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStats(t *testing.T) {
	assert := assert.New(t)

	pClient := &mocks.PClient{}
	iClient := &mocks.InfoClient{}

	localNodeID := ids.GenerateTestNodeID()
	subnetID := ids.GenerateTestID()

	startTime := time.Now()
	endTime := time.Now()
	weight := uint64(42)
	stake := uint64(42_000_000)
	conn := true

	remaining := ux.FormatDuration(endTime.Sub(startTime))

	reply := []platformvm.ClientPermissionlessValidator{
		{
			ClientStaker: platformvm.ClientStaker{
				StartTime:   uint64(startTime.Unix()),
				EndTime:     uint64(endTime.Unix()),
				NodeID:      localNodeID,
				Weight:      &weight,
				StakeAmount: &stake,
			},
			Connected: &conn,
		},
	}

	pClient.On("GetCurrentValidators", mock.Anything, mock.Anything, mock.Anything).Return(reply, nil)
	iClient.On("GetNodeID", mock.Anything).Return(localNodeID, nil)
	iClient.On("GetNodeVersion", mock.Anything).Return(&info.GetNodeVersionReply{
		VMVersions: map[string]string{
			subnetID.String(): "0.1.23",
		},
	}, nil)

	table := tablewriter.NewWriter(io.Discard)

	expectedVerStr := subnetID.String() + ": 0.1.23\n"

	rows, err := buildStats(pClient, iClient, table, subnetID)
	table.Append(rows[0])

	assert.NoError(err)
	assert.Equal(1, table.NumLines())
	assert.Equal(localNodeID.String(), rows[0][0])
	assert.Equal("true", rows[0][1])
	assert.Equal("42000000", rows[0][2])
	assert.Equal("42", rows[0][3])
	assert.Equal(remaining, rows[0][4])
	assert.Equal(expectedVerStr, rows[0][5])
}

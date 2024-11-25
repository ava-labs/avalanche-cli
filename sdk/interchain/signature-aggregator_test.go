// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/awm-relayer/peers/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var subnetID ids.ID

func instantiateAggregator(t *testing.T) (
	*SignatureAggregator,
	*mocks.MockAppRequestNetwork,
	error,
) {
	mockNetwork := mocks.NewMockAppRequestNetwork(gomock.NewController(t))
	subnetID = ids.GenerateTestID()
	aggregator, err := initSignatureAggregator(
		mockNetwork,
		logging.NoLog{},
		prometheus.DefaultRegisterer,
		subnetID,
		DefaultQuorumPercentage,
		time.Time{},
	)
	require.Equal(t, err, nil)
	return aggregator, mockNetwork, err
}

func TestSignatureAggregator(t *testing.T) {
	sa, _, err := instantiateAggregator(t)
	require.Nil(t, err)
	// basic checks
	require.Equal(t, sa.quorumPercentage, DefaultQuorumPercentage)
	require.Equal(t, sa.subnetID, subnetID)
	require.NotNil(t, sa.aggregator)
	msg, err := warp.NewUnsignedMessage(0, subnetID, []byte{})
	require.Nil(t, err)
	require.NotNil(t, msg)
}

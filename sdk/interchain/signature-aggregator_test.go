// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/awm-relayer/peers/mocks"
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
		subnetID,
		DefaultQuorumPercentage,
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

	// m := "0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000024c00000000053968786a235cbcfb6e57321b94378e95939b773a9626acf7a8cc440075c02c7268000002220000000000010000001452718d4ea91a6dd9a68940dbd687efa32315d11600000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000008db97c7cece249c2b98bdc0226cc4c2a57bf52fcb1d32d469938520383696931c26b9753662db74ad33c012f41e337aa828f1b74000000000000000000000000abcedf1234abcedf1234abcedf1234abcedf12340000000000000000000000000000000000000000000000000000000000002710000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000180000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000a100ff48a37cab9f87c8b5da933da46ea1a5fb80000000000000000000000000000000000000000000000000000000000000002acafebabecafebabecafebabecafebabecafebabecafebabecafebabecafebabecafebabecafebabecafe000000000000000000000000000000000000000000000000000000000000000000000000000000000000" //nolint:lll
	// _, err = sa.AggregateSignatures(m, hex.EncodeToString([]byte("test")))
	// require.Equal(t, err, nil)
}

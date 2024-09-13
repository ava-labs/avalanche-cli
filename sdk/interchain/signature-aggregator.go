// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	apiConfig "github.com/ava-labs/awm-relayer/config"
	"github.com/ava-labs/awm-relayer/peers"
	"github.com/ava-labs/awm-relayer/signature-aggregator/aggregator"
	"github.com/ava-labs/awm-relayer/signature-aggregator/config"
	"github.com/ava-labs/awm-relayer/signature-aggregator/metrics"
	awmTypes "github.com/ava-labs/awm-relayer/types"
	awmUtils "github.com/ava-labs/awm-relayer/utils"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DefaultQuorumPercentage   = uint64(67)
	DefaultSignatureCacheSize = uint64(1024 * 1024)
)

var etnaTime = time.Unix(0, 0)

type SignatureAggregator struct {
	subnetID         ids.ID
	quorumPercentage uint64
	aggregator       *aggregator.SignatureAggregator
}

func CreateAppRequestNetwork(network models.Network, logLevel logging.Level) (peers.AppRequestNetwork, error) {
	peerNetwork, err := peers.NewNetwork(
		logLevel,
		prometheus.DefaultRegisterer,
		nil,
		&config.Config{
			PChainAPI: &apiConfig.APIConfig{
				BaseURL: network.Endpoint,
			},
			InfoAPI: &apiConfig.APIConfig{
				BaseURL: network.Endpoint,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer network: %w", err)
	}
	return peerNetwork, nil
}

func NewSignatureAggregator(
	network peers.AppRequestNetwork,
	logger logging.Logger,
	subnetID ids.ID,
	quorumPercentage uint64,
) (*SignatureAggregator, error) {
	sa := &SignatureAggregator{}
	// set quorum percentage
	if quorumPercentage == 0 {
		sa.quorumPercentage = DefaultQuorumPercentage
	} else if quorumPercentage > 100 {
		return nil, fmt.Errorf("quorum percentage cannot be greater than 100")
	}
	sa.quorumPercentage = quorumPercentage

	sa.subnetID = subnetID

	messageCreator, err := message.NewCreator(
		logger,
		prometheus.DefaultRegisterer,
		constants.DefaultNetworkCompressionType,
		constants.DefaultNetworkMaximumInboundTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message creator: %w", err)
	}

	metricsInstance := metrics.NewSignatureAggregatorMetrics(prometheus.DefaultRegisterer)
	signatureAggregator, err := aggregator.NewSignatureAggregator(
		network,
		logger,
		DefaultSignatureCacheSize,
		metricsInstance,
		messageCreator,
		etnaTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature aggregator: %w", err)
	}
	sa.aggregator = signatureAggregator
	return sa, nil
}

func (s *SignatureAggregator) AggregateSignatures(
	msg string,
	justification string,
) (string, error) {
	// prepare message
	decodedMessage, err := hex.DecodeString(
		awmUtils.SanitizeHexString(msg),
	)
	if err != nil {
		return "", fmt.Errorf("failed to decode message: %w", err)
	}
	message, err := awmTypes.UnpackWarpMessage(decodedMessage)
	if err != nil {
		return "", fmt.Errorf("failed to unpack warp message: %w", err)
	}
	// prepare justification
	justificationBytes, err := hex.DecodeString(
		awmUtils.SanitizeHexString(justification),
	)
	if err != nil {
		return "", fmt.Errorf("failed to decode justification: %w", err)
	}
	// checks
	if awmUtils.IsEmptyOrZeroes(message.Bytes()) && awmUtils.IsEmptyOrZeroes(justificationBytes) {
		return "", fmt.Errorf("message and justification cannot be empty")
	}

	// aggregate signatures
	signedMessage, err := s.aggregator.CreateSignedMessage(
		message,
		justificationBytes,
		s.subnetID,
		s.quorumPercentage,
	)
	return hex.EncodeToString(signedMessage.Bytes()), err
}

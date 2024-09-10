// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package teleporter

import (
	"encoding/hex"
	"fmt"
	"time"

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
	DefaultQuorumPercentage   = 67
	DefaultSignatureCacheSize = uint64(1024 * 1024)
)

var (
	etnaTime = time.Unix(0, 0)
)

type SignatureAggregator struct {
	subnetID         ids.ID
	quorumPercentage uint64
	aggregator       *aggregator.SignatureAggregator
}

func NewSignatureAggregator(
	logger logging.Logger,
	logLevel logging.Level,
	subnetID string,
	quorumPercentage uint64,
) (*SignatureAggregator, error) {
	SignatureAggregator := &SignatureAggregator{}
	// set quorum percentage
	if quorumPercentage == 0 {
		SignatureAggregator.quorumPercentage = DefaultQuorumPercentage
	} else if quorumPercentage > 100 {
		return nil, fmt.Errorf("quorum percentage cannot be greater than 100")
	}
	SignatureAggregator.quorumPercentage = quorumPercentage

	// set subnet ID
	if subnetID == "" {
		return nil, fmt.Errorf("subnet ID cannot be empty")
	}
	signingSubnetID, err := awmUtils.HexOrCB58ToID(subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert subnet ID: %w", err)
	}
	SignatureAggregator.subnetID = signingSubnetID
	network, err := peers.NewNetwork(
		logLevel,
		prometheus.DefaultRegisterer,
		nil,
		&config.Config{
			LogLevel:  logLevel.String(),
			PChainAPI: &apiConfig.APIConfig{},
			InfoAPI:   &apiConfig.APIConfig{},
		},
	)

	messageCreator, err := message.NewCreator(
		logger,
		prometheus.DefaultRegisterer,
		constants.DefaultNetworkCompressionType,
		constants.DefaultNetworkMaximumInboundTimeout,
	)
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
	SignatureAggregator.aggregator = signatureAggregator
	return SignatureAggregator, nil

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

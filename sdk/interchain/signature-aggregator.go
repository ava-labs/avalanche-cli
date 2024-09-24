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

// createAppRequestNetwork creates a new AppRequestNetwork for the given network and log level.
//
// Parameters:
// - network: The network for which the AppRequestNetwork is created. It should be of type models.Network.
// - logLevel: The log level for the AppRequestNetwork. It should be of type logging.Level.
//
// Returns:
// - peers.AppRequestNetwork: The created AppRequestNetwork, or nil if an error occurred.
// - error: An error if the creation of the AppRequestNetwork failed.
func createAppRequestNetwork(network models.Network, logLevel logging.Level) (peers.AppRequestNetwork, error) {
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

// initSignatureAggregator initializes a new SignatureAggregator instance.
//
// network is the network to create the aggregator for.
// logger is the logger to use for logging.
// subnetID is the subnet ID to create the aggregator for.
// quorumPercentage is the quorum percentage to use for the aggregator.
//
// Returns a new SignatureAggregator instance, or an error if initialization fails.
func initSignatureAggregator(
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

// NewSignatureAggregator creates a new signature aggregator instance.
//
// network is the network to create the aggregator for.
// logger is the logger to use for logging.
// logLevel is the log level to use for logging.
// subnetID is the subnet ID to create the aggregator for.
// quorumPercentage is the quorum percentage to use for the aggregator.
//
// Returns a new signature aggregator instance, or an error if creation fails.
func NewSignatureAggregator(
	network models.Network,
	logger logging.Logger,
	logLevel logging.Level,
	subnetID ids.ID,
	quorumPercentage uint64,
) (*SignatureAggregator, error) {
	peerNetwork, err := createAppRequestNetwork(network, logLevel)
	if err != nil {
		return nil, err
	}
	return initSignatureAggregator(peerNetwork, logger, subnetID, quorumPercentage)
}

// AggregateSignatures aggregates signatures for a given message and justification.
//
// msg is the Hex encoded message to be signed
// justification is the hex encoded justification for the signature.
// Returns the signed message as a hexadecimal string, and an error if the operation fails.
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

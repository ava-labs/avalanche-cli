// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
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
func createAppRequestNetwork(
	network models.Network,
	logLevel logging.Level,
	registerer prometheus.Registerer,
	extraPeerEndpoints []info.Peer,
) (peers.AppRequestNetwork, error) {
	peerNetwork, err := peers.NewNetwork(
		logLevel,
		registerer,
		nil,
		extraPeerEndpoints,
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
	registerer prometheus.Registerer,
	subnetID ids.ID,
	quorumPercentage uint64,
	etnaTime time.Time,
) (*SignatureAggregator, error) {
	sa := &SignatureAggregator{}
	// set quorum percentage
	sa.quorumPercentage = quorumPercentage
	if quorumPercentage == 0 {
		sa.quorumPercentage = DefaultQuorumPercentage
	} else if quorumPercentage > 100 {
		return nil, fmt.Errorf("quorum percentage cannot be greater than 100")
	}
	sa.subnetID = subnetID

	messageCreator, err := message.NewCreator(
		logger,
		registerer,
		avagoconstants.DefaultNetworkCompressionType,
		avagoconstants.DefaultNetworkMaximumInboundTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message creator: %w", err)
	}

	metricsInstance := metrics.NewSignatureAggregatorMetrics(registerer)
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
	logLevel logging.Level,
	subnetID ids.ID,
	quorumPercentage uint64,
	extraPeerEndpoints []info.Peer,
) (*SignatureAggregator, error) {
	registerer := prometheus.NewRegistry()
	peerNetwork, err := createAppRequestNetwork(network, logLevel, registerer, extraPeerEndpoints)
	if err != nil {
		return nil, err
	}
	logger := logging.NewLogger(
		"init-aggregator",
		logging.NewWrappedCore(
			logLevel,
			os.Stdout,
			logging.JSON.ConsoleEncoder(),
		),
	)
	etnaTime := time.Time{}
	if t, ok := constants.EtnaActivationTime[network.ID]; ok {
		etnaTime = t
	}
	return initSignatureAggregator(peerNetwork, logger, registerer, subnetID, quorumPercentage, etnaTime)
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
	signedMessage, err := s.Sign(
		message,
		justificationBytes,
	)
	return hex.EncodeToString(signedMessage.Bytes()), err
}

// Sign aggregates signatures for a given message and justification.
//
// msg is the message to be signed
// justification is the justification for the signature.
// Returns the signed message, and an error if the operation fails.
func (s *SignatureAggregator) Sign(
	msg *warp.UnsignedMessage,
	justification []byte,
) (*warp.Message, error) {
	return s.aggregator.CreateSignedMessage(
		msg,
		justification,
		s.subnetID,
		s.quorumPercentage,
	)
}

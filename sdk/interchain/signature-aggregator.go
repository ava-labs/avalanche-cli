// // Copyright (C) 2025, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	apiConfig "github.com/ava-labs/icm-services/config"
	"github.com/ava-labs/icm-services/peers"
	"github.com/ava-labs/icm-services/signature-aggregator/aggregator"
	"github.com/ava-labs/icm-services/signature-aggregator/config"
	"github.com/ava-labs/icm-services/signature-aggregator/metrics"
	awmTypes "github.com/ava-labs/icm-services/types"
	awmUtils "github.com/ava-labs/icm-services/utils"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	DefaultQuorumPercentage   = uint64(67)
	DefaultSignatureCacheSize = uint64(1024 * 1024)
)

type SignatureAggregator struct {
	subnetID         ids.ID
	quorumPercentage uint64
	aggregator       *aggregator.SignatureAggregator
	network          peers.AppRequestNetwork
}

// createAppRequestNetwork creates a new AppRequestNetwork for the given network and log level.
//
// Parameters:
// - network: The network for which the AppRequestNetwork is created. It should be of type network.Network.
// - logLevel: The log level for the AppRequestNetwork. It should be of type logging.Level.
//
// Returns:
// - peers.AppRequestNetwork: The created AppRequestNetwork, or nil if an error occurred.
// - error: An error if the creation of the AppRequestNetwork failed.
func createAppRequestNetwork(
	network network.Network,
	logger logging.Logger,
	registerer prometheus.Registerer,
	extraPeerEndpoints []info.Peer,
	trackedSubnetIDs []string,
) (peers.AppRequestNetwork, error) {
	networkConfig := config.Config{
		PChainAPI: &apiConfig.APIConfig{
			BaseURL: network.Endpoint,
		},
		InfoAPI: &apiConfig.APIConfig{
			BaseURL: network.Endpoint,
		},
		AllowPrivateIPs:  true,
		TrackedSubnetIDs: trackedSubnetIDs,
	}
	if err := networkConfig.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate peer network config: %w", err)
	}
	peerNetwork, err := peers.NewNetwork(
		logger,
		registerer,
		networkConfig.GetTrackedSubnets(),
		extraPeerEndpoints,
		&networkConfig,
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
		constants.DefaultNetworkCompressionType,
		constants.DefaultNetworkMaximumInboundTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message creator: %w", err)
	}

	metricsInstance := metrics.NewSignatureAggregatorMetrics(registerer)
	signatureAggregator, err := aggregator.NewSignatureAggregator(
		network,
		logger,
		messageCreator,
		DefaultSignatureCacheSize,
		metricsInstance,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature aggregator: %w", err)
	}
	sa.aggregator = signatureAggregator
	sa.network = network
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
	ctx context.Context,
	network network.Network,
	logger logging.Logger,
	subnetID ids.ID,
	quorumPercentage uint64,
	extraPeerEndpoints []info.Peer,
) (*SignatureAggregator, error) {
	registerer := prometheus.NewRegistry()
	trackedSubnetIDs := []string{}
	if subnetID != constants.PrimaryNetworkID {
		trackedSubnetIDs = append(trackedSubnetIDs, subnetID.String())
	}
	peerNetwork, err := createAppRequestNetwork(network, logger, registerer, extraPeerEndpoints, trackedSubnetIDs)
	if err != nil {
		return nil, err
	}
	sa, err := initSignatureAggregator(peerNetwork, logger, registerer, subnetID, quorumPercentage)
	if err != nil {
		return sa, err
	}
	err = sa.waitForHealthy(ctx)
	return sa, err
}

func (s *SignatureAggregator) waitForHealthy(ctx context.Context) error {
	subnets := []ids.ID{}
	if s.subnetID != constants.PrimaryNetworkID {
		subnets = append(subnets, s.subnetID)
	}
	subnets = append(subnets, constants.PrimaryNetworkID)
	healthy := peers.GetNetworkHealthFunc(s.network, subnets)
	for {
		if err := healthy(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for signature aggregation being healthy: %w", ctx.Err())
		case <-time.After(1 * time.Second):
		}
	}
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
	if signed, err := s.aggregator.CreateSignedMessage(
		msg,
		justification,
		s.subnetID,
		s.quorumPercentage,
	); err == nil {
		return signed, nil
	}
	// many times first attempt just fails for connection timeouts (<= 10 secs spent there)
	return s.aggregator.CreateSignedMessage(
		msg,
		justification,
		s.subnetID,
		s.quorumPercentage,
	)
}

// SignatureAggregatorRunFile represents the run file structure for the signature aggregator
type SignatureAggregatorRunFile struct {
	Pid int `json:"pid"`
}

// AggregateSignaturesRequest represents the request structure for aggregating signatures
type AggregateSignaturesRequest struct {
	Message                string `json:"message"`
	Justification          string `json:"justification"`
	SigningSubnetID        string `json:"signing-subnet-id"`
	QuorumPercentage       int    `json:"quorum-percentage"`
	QuorumPercentageBuffer int    `json:"quorum-percentage-buffer,omitempty"`
}

// SignMessage sends a request to the signature aggregator to sign a message.
// It returns the signed warp message or an error if the operation fails.
func SignMessage(message, justification, signingSubnetID string, quorumPercentage int, logger logging.Logger, signatureAggregatorEndpoint string) (*warp.Message, error) {
	request := AggregateSignaturesRequest{
		Message:          message,
		Justification:    justification,
		SigningSubnetID:  signingSubnetID,
		QuorumPercentage: quorumPercentage,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Info("Calling signature aggregator",
		zap.String("request", string(requestBody)),
	)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post(
		signatureAggregatorEndpoint,
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		logger.Error("Error making request to signature aggregator",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Warn("Failed to close response body",
				zap.Error(closeErr),
			)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading response body",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Info("Received response from signature aggregator",
		zap.Int("status_code", resp.StatusCode),
		zap.String("response", string(body)),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signature aggregator returned non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the response to get the signed message hex
	var response struct {
		SignedMessage string `json:"signed-message"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Decode the hex string
	signedMessageBytes, err := hex.DecodeString(response.SignedMessage)
	if err != nil {
		return nil, fmt.Errorf("error decoding hex: %w", err)
	}

	// Parse the signed message
	signedMessage, err := warp.ParseMessage(signedMessageBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing signed message: %w", err)
	}

	return signedMessage, nil
}

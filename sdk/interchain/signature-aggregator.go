// // Copyright (C) 2025, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
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

// logWriter is a custom writer that forwards output to a logger
type logWriter struct {
	logger logging.Logger
	level  logging.Level
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	switch w.level {
	case logging.Info:
		w.logger.Info(msg)
	case logging.Error:
		w.logger.Error(msg)
	default:
		w.logger.Info(msg)
	}
	return len(p), nil
}

// StartSignatureAggregator starts the signature aggregator process.
// It handles port conflicts and retries if necessary.
func StartSignatureAggregator(binPath string, configPath string, logger logging.Logger) (int, error) {
	// Function to check if port is in use
	isPortInUse := func() bool {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err == nil {
			if err := conn.Close(); err != nil {
				logger.Warn("Failed to close connection while checking port",
					zap.Error(err),
				)
			}
			return true
		}
		return false
	}

	// Function to kill existing process
	killExistingProcess := func() error {
		// Try pkill first
		cmd := exec.Command("pkill", "-f", "signature-aggregator")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to kill existing signature-aggregator process: %w", err)
		}
		return nil
	}

	// Try to start the aggregator with retries
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if isPortInUse() {
			logger.Info("Port 8080 is in use, attempting to kill existing process",
				zap.Int("attempt", i+1),
				zap.Int("max_attempts", maxRetries),
			)
			if err := killExistingProcess(); err != nil {
				logger.Warn("Failed to kill existing process",
					zap.Error(err),
				)
			}
			// Wait for port to be released
			time.Sleep(2 * time.Second)
		}

		logger.Info("Starting Signature Aggregator",
			zap.Int("attempt", i+1),
			zap.Int("max_attempts", maxRetries),
		)

		cmd := exec.Command(binPath, "--config-file", configPath)
		cmd.Stdout = &logWriter{logger: logger, level: logging.Info}
		cmd.Stderr = &logWriter{logger: logger, level: logging.Error}

		if err := cmd.Start(); err != nil {
			if i == maxRetries-1 {
				return 0, fmt.Errorf("failed to start signature-aggregator after %d attempts: %w", maxRetries, err)
			}
			continue
		}

		ch := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			ch <- struct{}{}
		}()

		// Allow time for the aggregator to initialize
		time.Sleep(2 * time.Second)

		select {
		case <-ch:
			if i == maxRetries-1 {
				return 0, fmt.Errorf("signature-aggregator exited during startup after %d attempts", maxRetries)
			}
			continue
		default:
		}

		// Check if the aggregator is ready
		if err := waitForAggregatorReady("http://localhost:8080/aggregate-signatures", 5*time.Second); err != nil {
			_ = cmd.Process.Kill()
			if i == maxRetries-1 {
				return 0, fmt.Errorf("signature-aggregator not ready after %d attempts: %w", maxRetries, err)
			}
			continue
		}

		logger.Info("Signature Aggregator started successfully",
			zap.Int("attempt", i+1),
		)
		return cmd.Process.Pid, nil
	}

	return 0, fmt.Errorf("failed to start signature-aggregator after %d attempts", maxRetries)
}

// waitForAggregatorReady waits for the signature aggregator to be ready by checking its health endpoint.
func waitForAggregatorReady(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for signature-aggregator readiness")
		case <-ticker.C:
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("obtained error %s \n", err)
			}
			if err == nil && resp.StatusCode == http.StatusBadRequest {
				// A 400 means the service is up but received a malformed request
				if closeErr := resp.Body.Close(); closeErr != nil {
					fmt.Printf("Failed to close response body: %v\n", closeErr)
				}
				return nil
			}
		}
	}
}

// CreateSignatureAggregatorConfig creates a configuration for the signature aggregator.
func CreateSignatureAggregatorConfig(subnetID string, networkEndpoint string, peers []info.Peer) *SignatureAggregatorConfig {
	config := &SignatureAggregatorConfig{
		LogLevel:             "debug",
		PChainAPI:            APIConfig{BaseURL: networkEndpoint},
		InfoAPI:              APIConfig{BaseURL: networkEndpoint},
		SignatureCacheSize:   1048576,
		AllowPrivateIPs:      true,
		TrackedSubnetIDs:     []string{subnetID},
		ManuallyTrackedPeers: make([]PeerConfig, 0),
	}

	for _, peer := range peers {
		// Skip peers with invalid IP addresses
		if !peer.Info.PublicIP.IsValid() {
			continue
		}
		fmt.Printf("peer.Info.PublicIP.String() %s \n", peer.Info.PublicIP.String())
		config.ManuallyTrackedPeers = append(config.ManuallyTrackedPeers, PeerConfig{
			ID: peer.Info.ID.String(),
			IP: peer.Info.PublicIP.String(),
		})
	}

	return config
}

// WriteSignatureAggregatorConfig writes the signature aggregator configuration to a file.
func WriteSignatureAggregatorConfig(config *SignatureAggregatorConfig, configPath string) error {
	configBytes, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SignatureAggregatorConfig represents the configuration for the signature aggregator.
type SignatureAggregatorConfig struct {
	LogLevel             string       `json:"log-level"`
	PChainAPI            APIConfig    `json:"p-chain-api"`
	InfoAPI              APIConfig    `json:"info-api"`
	SignatureCacheSize   int          `json:"signature-cache-size"`
	AllowPrivateIPs      bool         `json:"allow-private-ips"`
	TrackedSubnetIDs     []string     `json:"tracked-subnet-ids"`
	ManuallyTrackedPeers []PeerConfig `json:"manually-tracked-peers"`
}

// APIConfig represents the configuration for an API endpoint.
type APIConfig struct {
	BaseURL string `json:"base-url"`
}

// PeerConfig represents the configuration for a peer.
type PeerConfig struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

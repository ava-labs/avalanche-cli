// // Copyright (C) 2025, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"
)

const (
	DefaultQuorumPercentage   = uint64(67)
	DefaultSignatureCacheSize = uint64(1024 * 1024)
	MaxRetries                = 3
	InitialBackoff            = 1 * time.Second
)

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
		SigningSubnetID:  signingSubnetID,
		QuorumPercentage: quorumPercentage,
	}
	request.Justification = justification

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

	var lastErr error
	backoff := InitialBackoff

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			logger.Info("Retrying signature aggregator request",
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoff),
			)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		resp, err := client.Post(
			signatureAggregatorEndpoint,
			"application/json",
			bytes.NewBuffer(requestBody),
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to make request: %w", err)
			logger.Error("Error making request to signature aggregator",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Warn("Failed to close response body",
				zap.Error(closeErr),
			)
		}
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			logger.Error("Error reading response body",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		logger.Info("Received response from signature aggregator",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
			zap.Int("attempt", attempt+1),
		)

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("signature aggregator returned non-200 status code: %d, body: %s", resp.StatusCode, string(body))
			logger.Error("Received non-200 status code",
				zap.Int("status_code", resp.StatusCode),
				zap.String("body", string(body)),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		// Parse the response to get the signed message hex
		var response struct {
			SignedMessage string `json:"signed-message"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			logger.Error("Error parsing response",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		// Decode the hex string
		signedMessageBytes, err := hex.DecodeString(response.SignedMessage)
		if err != nil {
			lastErr = fmt.Errorf("error decoding hex: %w", err)
			logger.Error("Error decoding hex",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		// Parse the signed message
		signedMessage, err := warp.ParseMessage(signedMessageBytes)
		if err != nil {
			lastErr = fmt.Errorf("error parsing signed message: %w", err)
			logger.Error("Error parsing signed message",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
			continue
		}

		return signedMessage, nil
	}

	return nil, fmt.Errorf("failed after %d attempts, last error: %w", MaxRetries, lastErr)
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package teleporter

import (
	"encoding/hex"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/awm-relayer/signature-aggregator/aggregator"
	awmTypes "github.com/ava-labs/awm-relayer/types"
	awmUtils "github.com/ava-labs/awm-relayer/utils"
)

const (
	DefaultQuorumPercentage = 67
)

type SignatureAggregator struct {
	subnetID         ids.ID
	quorumPercentage uint64
	aggregator       *aggregator.SignatureAggregator
}

func NewSignatureAggregator(subnetID string, quorumPercentage uint64) (*SignatureAggregator, error) {
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
	if awmUtils.IsEmptyOrZeroes(message.Bytes()) && awmUtils.IsEmptyOrZeroes(justification) {
		return "", fmt.Errorf("message and justification cannot be empty")
	}

	// aggregate signatures
	signedMessage, err := s.aggregator.CreateSignedMessage(
		message,
		justificationBytes,
		s.subnetID,
		s.quorumPercentage,
	)
	return hex.EncodeToString(signedMessage.Bytes()), nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func NewBlsSecretKeyBytes() ([]byte, error) {
	blsSignerKey, err := bls.NewSecretKey()
	if err != nil {
		return nil, err
	}
	return bls.SecretKeyToBytes(blsSignerKey), nil
}

func ToNodeID(certBytes []byte) (ids.NodeID, error) {
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return ids.EmptyNodeID, fmt.Errorf("failed to decode certificate")
	}
	cert, err := staking.ParseCertificate(block.Bytes)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.NodeIDFromCert(cert), nil
}

func ToBLSPoP(keyBytes []byte) (
	[]byte, // bls public key
	[]byte, // bls proof of possession
	error,
) {
	sk, err := bls.SecretKeyFromBytes(keyBytes)
	if err != nil {
		return nil, nil, err
	}
	pop := signer.NewProofOfPossession(sk)
	return pop.PublicKey[:], pop.ProofOfPossession[:], nil
}

// GetNodeParams returns node id, bls public key and bls proof of possession
func GetNodeParams(nodeDir string) (
	ids.NodeID,
	[]byte, // bls public key
	[]byte, // bls proof of possession
	error,
) {
	certBytes, err := os.ReadFile(filepath.Join(nodeDir, constants.StakerCertFileName))
	if err != nil {
		return ids.EmptyNodeID, nil, nil, err
	}
	nodeID, err := ToNodeID(certBytes)
	if err != nil {
		return ids.EmptyNodeID, nil, nil, err
	}
	blsKeyBytes, err := os.ReadFile(filepath.Join(nodeDir, constants.BLSKeyFileName))
	if err != nil {
		return ids.EmptyNodeID, nil, nil, err
	}
	blsPub, blsPoP, err := ToBLSPoP(blsKeyBytes)
	if err != nil {
		return ids.EmptyNodeID, nil, nil, err
	}
	return nodeID, blsPub, blsPoP, nil
}

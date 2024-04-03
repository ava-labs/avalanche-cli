// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

func NewBlsSecretKeyBytes() ([]byte, error) {
	blsSignerKey, err := bls.NewSecretKey()
	if err != nil {
		return nil, err
	}
	return bls.SecretKeyToBytes(blsSignerKey), nil
}

func ToNodeID(certBytes []byte, keyBytes []byte) (ids.NodeID, error) {
	tlsCert, err := staking.LoadTLSCertFromBytes(keyBytes, certBytes)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	cert, err := staking.ParseCertificate(tlsCert.Leaf.Raw)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.NodeIDFromCert(cert), nil
}

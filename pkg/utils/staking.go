package utils

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

func NewStakingCertAndKeyBytes() ([]byte, []byte, error) {
	cBytes, kBytes, err := staking.NewCertAndKeyBytes()
	if err != nil {
		return nil, nil, err
	}
	return cBytes, kBytes, nil
}

func NewBlsSignerCertAndKeyBytes() ([]byte, []byte, error) {
	blsSignerKey, err := bls.NewSecretKey()
	if err != nil {
		return nil, nil, err
	}
	blsPublicBytes := bls.PublicKeyToBytes(bls.PublicFromSecretKey(blsSignerKey))
	return bls.SecretKeyToBytes(blsSignerKey), blsPublicBytes, nil
}

func GetNodeID(certBytes []byte, keyBytes []byte) (string, error) {
	cert, err := staking.LoadTLSCertFromBytes(keyBytes, certBytes)
	if err != nil {
		return "", err
	}
	return ids.NodeIDFromCert(staking.CertificateFromX509(cert.Leaf)).String(), nil
}

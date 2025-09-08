// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package key

import (
	"bytes"
	"errors"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

const (
	ewoqPChainAddr    = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	fallbackNetworkID = 999999 // unaffiliated networkID should trigger HRP Fallback
)

func TestNewKeyEwoq(t *testing.T) {
	t.Parallel()

	m, err := NewSoft(
		WithPrivateKeyEncoded(EwoqPrivateKey),
	)
	if err != nil {
		t.Fatal(err)
	}
	network := models.NewNetwork(0, fallbackNetworkID, "", "")
	pChainAddrStr, err := m.GetNetworkChainAddress(network, "P")
	if err != nil {
		t.Fatal(err)
	}
	if pChainAddrStr[0] != ewoqPChainAddr {
		t.Fatalf("unexpected P-Chain address %q, expected %q", pChainAddrStr, ewoqPChainAddr)
	}

	keyPath := filepath.Join(t.TempDir(), "key.pk")
	if err := m.Save(keyPath); err != nil {
		t.Fatal(err)
	}

	m2, err := LoadSoft(fallbackNetworkID, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(m.PrivKeyRaw(), m2.PrivKeyRaw()) {
		t.Fatalf("loaded key unexpected %v, expected %v", m2.PrivKeyRaw(), m.PrivKeyRaw())
	}
}

func TestNewKey(t *testing.T) {
	t.Parallel()

	skBytes, err := cb58.Decode(rawEwoqPk)
	if err != nil {
		t.Fatal(err)
	}
	ewoqPk, err := secp256k1.ToPrivateKey(skBytes)
	if err != nil {
		t.Fatal(err)
	}

	privKey2, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	tt := []struct {
		name   string
		opts   []SOpOption
		expErr error
	}{
		{
			name:   "test",
			opts:   nil,
			expErr: nil,
		},
		{
			name: "ewop with WithPrivateKey",
			opts: []SOpOption{
				WithPrivateKey(ewoqPk),
			},
			expErr: nil,
		},
		{
			name: "ewop with WithPrivateKeyEncoded",
			opts: []SOpOption{
				WithPrivateKeyEncoded(EwoqPrivateKey),
			},
			expErr: nil,
		},
		{
			name: "ewop with WithPrivateKey/WithPrivateKeyEncoded",
			opts: []SOpOption{
				WithPrivateKey(ewoqPk),
				WithPrivateKeyEncoded(EwoqPrivateKey),
			},
			expErr: nil,
		},
		{
			name: "ewop with invalid WithPrivateKey",
			opts: []SOpOption{
				WithPrivateKey(privKey2),
				WithPrivateKeyEncoded(EwoqPrivateKey),
			},
			expErr: ErrInvalidPrivateKey,
		},
	}
	for i, tv := range tt {
		_, err := NewSoft(tv.opts...)
		if !errors.Is(err, tv.expErr) {
			t.Fatalf("#%d(%s): unexpected error %v, expected %v", i, tv.name, err, tv.expErr)
		}
	}
}

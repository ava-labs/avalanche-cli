// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package key

import (
	"bytes"
	"errors"
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
		fallbackNetworkID,
		WithPrivateKeyEncoded(EwoqPrivateKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	if m.P()[0] != ewoqPChainAddr {
		t.Fatalf("unexpected P-Chain address %q, expected %q", m.P(), ewoqPChainAddr)
	}

	keyPath := filepath.Join(t.TempDir(), "key.pk")
	if err := m.Save(keyPath); err != nil {
		t.Fatal(err)
	}

	m2, err := LoadSoft(fallbackNetworkID, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(m.Raw(), m2.Raw()) {
		t.Fatalf("loaded key unexpected %v, expected %v", m2.Raw(), m.Raw())
	}
}

func TestNewKey(t *testing.T) {
	t.Parallel()

	skBytes, err := cb58.Decode(rawEwoqPk)
	if err != nil {
		t.Fatal(err)
	}
	factory := &secp256k1.Factory{}
	ewoqPk, err := factory.ToPrivateKey(skBytes)
	if err != nil {
		t.Fatal(err)
	}

	privKey2, err := factory.NewPrivateKey()
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
		_, err := NewSoft(fallbackNetworkID, tv.opts...)
		if !errors.Is(err, tv.expErr) {
			t.Fatalf("#%d(%s): unexpected error %v, expected %v", i, tv.name, err, tv.expErr)
		}
	}
}

// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package key

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

const ewoqPChainAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"

func TestNewKeyEwoq(t *testing.T) {
	t.Parallel()

	m, err := NewSoft(
		WithPrivateKeyEncoded(EwoqPrivateKey),
	)
	require.NoError(t, err)

	pAddr, err := m.P("custom")
	require.NoError(t, err)
	require.Equal(t, ewoqPChainAddr, pAddr)
	keyPath := filepath.Join(t.TempDir(), "key.pk")
	require.NoError(t, m.Save(keyPath))
	m2, err := LoadSoft(keyPath)
	require.NoError(t, err)
	require.Equal(t, m.PrivKeyRaw(), m2.PrivKeyRaw())
}

func TestNewKey(t *testing.T) {
	t.Parallel()

	skBytes, err := cb58.Decode(rawEwoqPk)
	require.NoError(t, err)

	ewoqPk, err := secp256k1.ToPrivateKey(skBytes)
	require.NoError(t, err)

	privKey2, err := secp256k1.NewPrivateKey()
	require.NoError(t, err)

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
	for _, tv := range tt {
		_, err := NewSoft(tv.opts...)
		require.ErrorIs(t, err, tv.expErr)
	}
}

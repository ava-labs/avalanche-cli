// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package key

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	eth_crypto "github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
)

var (
	ErrInvalidPrivateKey         = errors.New("invalid private key")
	ErrInvalidPrivateKeyLen      = errors.New("invalid private key length (expect 64 bytes in hex)")
	ErrInvalidPrivateKeyEnding   = errors.New("invalid private key ending")
	ErrInvalidPrivateKeyEncoding = errors.New("invalid private key encoding")
)

var _ Key = &SoftKey{}

type SoftKey struct {
	privKey        *secp256k1.PrivateKey
	privKeyRaw     []byte
	privKeyEncoded string

	pAddr string
	xAddr string

	keyChain *secp256k1fx.Keychain
}

const (
	privKeyEncPfx = "PrivateKey-"
	privKeySize   = 64

	rawEwoqPk      = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	EwoqPrivateKey = privKeyEncPfx + rawEwoqPk
)

var (
	keyFactory   = new(secp256k1.Factory)
	ewoqKeyBytes = []byte("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
)

type SOp struct {
	privKey        *secp256k1.PrivateKey
	privKeyEncoded string
}

type SOpOption func(*SOp)

func (sop *SOp) applyOpts(opts []SOpOption) {
	for _, opt := range opts {
		opt(sop)
	}
}

// To create a new key SoftKey with a pre-loaded private key.
func WithPrivateKey(privKey *secp256k1.PrivateKey) SOpOption {
	return func(sop *SOp) {
		sop.privKey = privKey
	}
}

// To create a new key SoftKey with a pre-defined private key.
func WithPrivateKeyEncoded(privKey string) SOpOption {
	return func(sop *SOp) {
		sop.privKeyEncoded = privKey
	}
}

func NewSoft(networkID uint32, opts ...SOpOption) (*SoftKey, error) {
	ret := &SOp{}
	ret.applyOpts(opts)

	// set via "WithPrivateKeyEncoded"
	if len(ret.privKeyEncoded) > 0 {
		privKey, err := decodePrivateKey(ret.privKeyEncoded)
		if err != nil {
			return nil, err
		}
		// to not overwrite
		if ret.privKey != nil &&
			!bytes.Equal(ret.privKey.Bytes(), privKey.Bytes()) {
			return nil, ErrInvalidPrivateKey
		}
		ret.privKey = privKey
	}

	// generate a new one
	if ret.privKey == nil {
		var err error
		ret.privKey, err = keyFactory.NewPrivateKey()
		if err != nil {
			return nil, err
		}
	}

	privKey := ret.privKey
	privKeyEncoded, err := encodePrivateKey(ret.privKey)
	if err != nil {
		return nil, err
	}

	// double-check encoding is consistent
	if ret.privKeyEncoded != "" &&
		ret.privKeyEncoded != privKeyEncoded {
		return nil, ErrInvalidPrivateKeyEncoding
	}

	keyChain := secp256k1fx.NewKeychain()
	keyChain.Add(privKey)

	m := &SoftKey{
		privKey:        privKey,
		privKeyRaw:     privKey.Bytes(),
		privKeyEncoded: privKeyEncoded,

		keyChain: keyChain,
	}

	// Parse HRP to create valid address
	hrp := GetHRP(networkID)
	m.pAddr, err = address.Format("P", hrp, m.privKey.PublicKey().Address().Bytes())
	if err != nil {
		return nil, err
	}
	m.xAddr, err = address.Format("X", hrp, m.privKey.PublicKey().Address().Bytes())
	if err != nil {
		return nil, err
	}

	return m, nil
}

// LoadSoft loads the private key from disk and creates the corresponding SoftKey.
func LoadSoft(networkID uint32, keyPath string) (*SoftKey, error) {
	kb, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return LoadSoftFromBytes(networkID, kb)
}

func LoadEwoq(networkID uint32) (*SoftKey, error) {
	ux.Logger.PrintToUser("Loading EWOQ key")
	return LoadSoftFromBytes(networkID, ewoqKeyBytes)
}

// LoadSoftFromBytes loads the private key from bytes and creates the corresponding SoftKey.
func LoadSoftFromBytes(networkID uint32, kb []byte) (*SoftKey, error) {
	// in case, it's already encoded
	k, err := NewSoft(networkID, WithPrivateKeyEncoded(string(kb)))
	if err == nil {
		return k, nil
	}

	r := bufio.NewReader(bytes.NewBuffer(kb))
	buf := make([]byte, privKeySize)
	n, err := readASCII(buf, r)
	if err != nil {
		return nil, err
	}
	if n != len(buf) {
		return nil, ErrInvalidPrivateKeyLen
	}
	if err := checkKeyFileEnd(r); err != nil {
		return nil, err
	}

	skBytes, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}
	privKey, err := keyFactory.ToPrivateKey(skBytes)
	if err != nil {
		return nil, err
	}

	return NewSoft(networkID, WithPrivateKey(privKey))
}

// readASCII reads into 'buf', stopping when the buffer is full or
// when a non-printable control character is encountered.
func readASCII(buf []byte, r io.ByteReader) (n int, err error) {
	for ; n < len(buf); n++ {
		buf[n], err = r.ReadByte()
		switch {
		case errors.Is(err, io.EOF) || buf[n] < '!':
			return n, nil
		case err != nil:
			return n, err
		}
	}
	return n, nil
}

const fileEndLimit = 1

// checkKeyFileEnd skips over additional newlines at the end of a key file.
func checkKeyFileEnd(r io.ByteReader) error {
	for idx := 0; ; idx++ {
		b, err := r.ReadByte()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case b != '\n' && b != '\r':
			return ErrInvalidPrivateKeyEnding
		case idx > fileEndLimit:
			return ErrInvalidPrivateKeyLen
		}
	}
}

func encodePrivateKey(pk *secp256k1.PrivateKey) (string, error) {
	privKeyRaw := pk.Bytes()
	enc, err := cb58.Encode(privKeyRaw)
	if err != nil {
		return "", err
	}
	return privKeyEncPfx + enc, nil
}

func decodePrivateKey(enc string) (*secp256k1.PrivateKey, error) {
	rawPk := strings.Replace(enc, privKeyEncPfx, "", 1)
	skBytes, err := cb58.Decode(rawPk)
	if err != nil {
		return nil, err
	}
	privKey, err := keyFactory.ToPrivateKey(skBytes)
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

func (m *SoftKey) C() string {
	ecdsaPrv := m.privKey.ToECDSA()
	pub := ecdsaPrv.PublicKey

	addr := eth_crypto.PubkeyToAddress(pub)
	return addr.String()
}

// Returns the KeyChain
func (m *SoftKey) KeyChain() *secp256k1fx.Keychain {
	return m.keyChain
}

// Returns the private key.
func (m *SoftKey) Key() *secp256k1.PrivateKey {
	return m.privKey
}

// Returns the private key in raw bytes.
func (m *SoftKey) Raw() []byte {
	return m.privKeyRaw
}

// Returns the private key encoded in CB58 and "PrivateKey-" prefix.
func (m *SoftKey) Encode() string {
	return m.privKeyEncoded
}

// Saves the private key to disk with hex encoding.
func (m *SoftKey) Save(p string) error {
	k := hex.EncodeToString(m.privKeyRaw)
	return os.WriteFile(p, []byte(k), constants.WriteReadUserOnlyPerms)
}

func (m *SoftKey) P() []string {
	return []string{m.pAddr}
}

func (m *SoftKey) X() []string {
	return []string{m.xAddr}
}

func (m *SoftKey) Spends(outputs []*avax.UTXO, opts ...OpOption) (
	totalBalanceToSpend uint64,
	inputs []*avax.TransferableInput,
	signers [][]ids.ShortID,
) {
	ret := &Op{}
	ret.applyOpts(opts)

	for _, out := range outputs {
		input, psigners, err := m.spend(out, ret.time)
		if err != nil {
			zap.L().Warn("cannot spend with current key", zap.Error(err))
			continue
		}
		totalBalanceToSpend += input.Amount()
		inputs = append(inputs, &avax.TransferableInput{
			UTXOID: out.UTXOID,
			Asset:  out.Asset,
			In:     input,
		})
		// Convert to ids.ShortID to adhere with interface
		pksigners := make([]ids.ShortID, len(psigners))
		for i, psigner := range psigners {
			pksigners[i] = psigner.PublicKey().Address()
		}
		signers = append(signers, pksigners)
		if ret.targetAmount > 0 &&
			totalBalanceToSpend > ret.targetAmount+ret.feeDeduct {
			break
		}
	}
	SortTransferableInputsWithSigners(inputs, signers)
	return totalBalanceToSpend, inputs, signers
}

func (m *SoftKey) spend(output *avax.UTXO, time uint64) (
	input avax.TransferableIn,
	signers []*secp256k1.PrivateKey,
	err error,
) {
	// "time" is used to check whether the key owner
	// is still within the lock time (thus can't spend).
	inputf, psigners, err := m.keyChain.Spend(output.Out, time)
	if err != nil {
		return nil, nil, err
	}
	var ok bool
	input, ok = inputf.(avax.TransferableIn)
	if !ok {
		return nil, nil, ErrInvalidType
	}
	return input, psigners, nil
}

func (m *SoftKey) Addresses() []ids.ShortID {
	return []ids.ShortID{m.privKey.PublicKey().Address()}
}

func (m *SoftKey) Sign(pTx *txs.Tx, signers [][]ids.ShortID) error {
	privsigners := make([][]*secp256k1.PrivateKey, len(signers))
	for i, inputSigners := range signers {
		privsigners[i] = make([]*secp256k1.PrivateKey, len(inputSigners))
		for j, signer := range inputSigners {
			if signer != m.privKey.PublicKey().Address() {
				// Should never happen
				return ErrCantSpend
			}
			privsigners[i][j] = m.privKey
		}
	}

	return pTx.Sign(txs.Codec, privsigners)
}

func (m *SoftKey) Match(owners *secp256k1fx.OutputOwners, time uint64) ([]uint32, []ids.ShortID, bool) {
	indices, privs, ok := m.keyChain.Match(owners, time)
	pks := make([]ids.ShortID, len(privs))
	for i, priv := range privs {
		pks[i] = priv.PublicKey().Address()
	}
	return indices, pks, ok
}

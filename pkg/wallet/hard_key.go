// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package wallet

const (
	numAddresses = 1024
)

/*
var _ Key = &HardKey{}

type HardKey struct {
	l *ledger.Ledger

	pAddrs       []string
	shortAddrs   []ids.ShortID
	shortAddrMap map[ids.ShortID]uint32
}

func parseLedgerErr(err error, fallback string) {
	errString := err.Error()
	switch {
	case strings.Contains(errString, "LedgerHID device") && strings.Contains(errString, "not found"):
		// color.Outf("{{red}}ledger is not connected{{/}}\n")
		ux.Logger.PrintToUser("ledger is not connected")
	case strings.Contains(errString, "6b0c"):
		// color.Outf("{{red}}ledger is not unlocked{{/}}\n")
		ux.Logger.PrintToUser("ledger is not unlocked")
	case strings.Contains(errString, "APDU_CODE_CONDITIONS_NOT_SATISFIED"):
		// color.Outf("{{red}}ledger rejected signing{{/}}\n")
		ux.Logger.PrintToUser("ledger rejected signing")
	default:
		// color.Outf("{{red}}%s: %v{{/}}\n", fallback, err)
		ux.Logger.PrintToUser("%s: %v", fallback, err)
	}
}

// retriableLedgerAction wraps all Ledger calls to allow the user to try and
// recover instead of exiting (in case their Ledger locks).
func retriableLegerAction(f func() error, fallback string) error {
	for {
		rerr := f()
		if rerr == nil {
			return nil
		}
		parseLedgerErr(rerr, fallback)

		ux.Logger.PrintToUser("ledger action failed")
		// color.Outf("\n{{cyan}}ledger action failed...what now?{{/}}\n")
		prompt := promptui.Select{
			Label:  "\n",
			Stdout: os.Stdout,
			Items: []string{
				formatter.F("{{green}}retry{{/}}"),
				formatter.F("{{red}}exit{{/}}"),
			},
		}
		idx, _, err := prompt.Run()
		if err != nil || idx == 1 {
			return rerr
		}
	}
}

func NewHard(networkID uint32) (*HardKey, error) {
	k := &HardKey{}
	ux.Logger.PrintToUser("connecting to ledger...")
	// color.Outf("{{yellow}}connecting to ledger...{{/}}\n")
	if err := retriableLegerAction(func() error {
		l, err := ledger.Connect()
		if err != nil {
			return err
		}
		k.l = l
		return nil
	}, "failed to connect to ledger"); err != nil {
		return nil, err
	}

	ux.Logger.PrintToUser("deriving address from ledger...")
	// color.Outf("{{yellow}}deriving address from ledger...{{/}}\n")
			// TODO: This does not compile due to outdated ledger lib
		hrp := getHRP(networkID)
		if err := retriableLegerAction(func() error {
			addrs, err := k.l.Addresses(hrp, numAddresses)
			if err != nil {
				return err
			}
			laddrs := len(addrs)
			k.pAddrs = make([]string, laddrs)
			k.shortAddrs = make([]ids.ShortID, laddrs)
			k.shortAddrMap = map[ids.ShortID]uint32{}
			for i, addr := range addrs {
				k.pAddrs[i], err = address.Format("P", hrp, addr.ShortAddr[:])
				if err != nil {
					return err
				}
				k.shortAddrs[i] = addr.ShortAddr
				k.shortAddrMap[addr.ShortAddr] = uint32(i)
			}
			return nil
		}, "failed to get extended public key"); err != nil {
			return nil, err
		}

	ux.Logger.PrintToUser("derived primary address from ledger: %s", k.pAddrs[0])
	// color.Outf("{{yellow}}derived primary address from ledger: %s{{/}}\n", k.pAddrs[0])
	return k, nil
}

func (h *HardKey) Disconnect() error {
	return h.l.Disconnect()
}

func (h *HardKey) P() []string { return h.pAddrs }

func (h *HardKey) Addresses() []ids.ShortID {
	return h.shortAddrs
}

// Sign transaction with the Ledger private key
//
// This is a slightly modified version of *platformvm.Tx.Sign().
func (h *HardKey) Sign(pTx *platformvm.Tx, signers [][]ids.ShortID) error {
	unsignedBytes, err := platformvm.Codec.Marshal(platformvm.CodecVersion, &pTx.UnsignedTx)
	if err != nil {
		return fmt.Errorf("couldn't marshal UnsignedTx: %w", err)
	}
	hash := hashing.ComputeHash256(unsignedBytes)

	// Generate signature
	uniqueSigners := map[uint32]struct{}{}
	for _, inputSigners := range signers {
		for _, signer := range inputSigners {
			if v, ok := h.shortAddrMap[signer]; ok {
				uniqueSigners[v] = struct{}{}
			} else {
				// Should never happen
				return ErrCantSpend
			}
		}
	}
	indices := make([]uint32, 0, len(uniqueSigners))
	for idx := range uniqueSigners {
		indices = append(indices, idx)
	}

	var sigs [][]byte
	if err := retriableLegerAction(func() error {
		sigs, err = h.l.SignHash(hash, indices)
		if err != nil {
			return err
		}
		return nil
	}, "failed to sign hash"); err != nil {
		return fmt.Errorf("problem generating signatures: %w", err)
	}
	sigMap := map[ids.ShortID][]byte{}
	for i, idx := range indices {
		sigMap[h.shortAddrs[idx]] = sigs[i]
	}

	// Add credentials to transaction
	for _, inputSigners := range signers {
		cred := &secp256k1fx.Credential{
			Sigs: make([][crypto.SECP256K1RSigLen]byte, len(inputSigners)),
		}
		for i, signer := range inputSigners {
			copy(cred.Sigs[i][:], sigMap[signer])
		}
		pTx.Creds = append(pTx.Creds, cred)
	}

	// Create signed tx bytes
	signedBytes, err := platformvm.Codec.Marshal(platformvm.CodecVersion, pTx)
	if err != nil {
		return fmt.Errorf("couldn't marshal ProposalTx: %w", err)
	}
	pTx.Initialize(unsignedBytes, signedBytes)
	return nil
}
*/

/*
func (h *HardKey) Spends(outputs []*avax.UTXO, opts ...OpOption) (
	totalBalanceToSpend uint64,
	inputs []*avax.TransferableInput,
	signers [][]ids.ShortID,
) {
	ret := &Op{}
	ret.applyOpts(opts)

	for _, out := range outputs {
		input, txsigners, err := h.spend(out, ret.time)
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
		signers = append(signers, txsigners)
		if ret.targetAmount > 0 &&
			totalBalanceToSpend > ret.targetAmount+ret.feeDeduct {
			break
		}
	}
	SortTransferableInputsWithSigners(inputs, signers)
	return totalBalanceToSpend, inputs, signers
}

func (h *HardKey) spend(output *avax.UTXO, time uint64) (
	input avax.TransferableIn,
	signers []ids.ShortID,
	err error,
) {
	// "time" is used to check whether the key owner
	// is still within the lock time (thus can't spend).
	inputf, signers, err := h.lspend(output.Out, time)
	if err != nil {
		return nil, nil, err
	}
	var ok bool
	input, ok = inputf.(avax.TransferableIn)
	if !ok {
		return nil, nil, ErrInvalidType
	}
	return input, signers, nil
}

func (h *HardKey) lspend(out verify.Verifiable, time uint64) (verify.Verifiable, []ids.ShortID, error) {
	switch out := out.(type) {
	case *secp256k1fx.MintOutput:
		if sigIndices, signers, able := h.Match(&out.OutputOwners, time); able {
			return &secp256k1fx.Input{
				SigIndices: sigIndices,
			}, signers, nil
		}
		return nil, nil, ErrCantSpend
	case *secp256k1fx.TransferOutput:
		if sigIndices, signers, able := h.Match(&out.OutputOwners, time); able {
			return &secp256k1fx.TransferInput{
				Amt: out.Amt,
				Input: secp256k1fx.Input{
					SigIndices: sigIndices,
				},
			}, signers, nil
		}
		return nil, nil, ErrCantSpend
	}
	return nil, nil, fmt.Errorf("can't spend UTXO because it is unexpected type %T", out)
}

func (h *HardKey) Match(owners *secp256k1fx.OutputOwners, time uint64) ([]uint32, []ids.ShortID, bool) {
	if time < owners.Locktime {
		return nil, nil, false
	}
	sigs := make([]uint32, 0, owners.Threshold)
	signers := make([]ids.ShortID, 0, owners.Threshold)
	for i := uint32(0); i < uint32(len(owners.Addrs)) && uint32(len(sigs)) < owners.Threshold; i++ {
		if _, ok := h.shortAddrMap[owners.Addrs[i]]; ok {
			sigs = append(sigs, i)
			signers = append(signers, owners.Addrs[i])
		}
	}
	return sigs, signers, uint32(len(sigs)) == owners.Threshold
}
*/

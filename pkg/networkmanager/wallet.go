// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package server

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/chain/p"
	"github.com/ava-labs/avalanchego/wallet/chain/x"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
)

const defaultTimeout = time.Minute

func createDefaultCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

type refreshableWallet struct {
	primary.Wallet
	kc *secp256k1fx.Keychain

	pBackend p.Backend
	pBuilder p.Builder
	pSigner  p.Signer

	xBackend x.Backend
	xBuilder x.Builder
	xSigner  x.Signer

	httpRPCEp string
}

// Creates a new wallet to work around the case where the new wallet object
// is not able to find previous transactions in the cache.
// TODO: support tx backfilling in upstream wallet SDK.
func createRefreshableWallet(ctx context.Context, httpRPCEp string, kc *secp256k1fx.Keychain) (*refreshableWallet, error) {
	cctx, cancel := createDefaultCtx(ctx)
	pCTX, xCTX, utxos, err := primary.FetchState(cctx, httpRPCEp, kc.Addrs)
	cancel()
	if err != nil {
		return nil, err
	}

	pUTXOs := primary.NewChainUTXOs(constants.PlatformChainID, utxos)
	pTXs := make(map[ids.ID]*platformvm.Tx)
	pBackend := p.NewBackend(pCTX, pUTXOs, pTXs)
	pBuilder := p.NewBuilder(kc.Addrs, pBackend)
	pSigner := p.NewSigner(kc, pBackend)

	// need updates when reconnected
	pClient := platformvm.NewClient(httpRPCEp)
	pw := p.NewWallet(pBuilder, pSigner, pClient, pBackend)

	xChainID := xCTX.BlockchainID()
	xUTXOs := primary.NewChainUTXOs(xChainID, utxos)
	xBackend := x.NewBackend(xCTX, xChainID, xUTXOs)
	xBuilder := x.NewBuilder(kc.Addrs, xBackend)
	xSigner := x.NewSigner(kc, xBackend)

	// need updates when reconnected
	xClient := avm.NewClient(httpRPCEp, "X")
	xw := x.NewWallet(xBuilder, xSigner, xClient, xBackend)

	return &refreshableWallet{
		Wallet: primary.NewWallet(pw, xw),
		kc:     kc,

		pBackend: pBackend,
		pBuilder: pBuilder,
		pSigner:  pSigner,

		xBackend: xBackend,
		xBuilder: xBuilder,
		xSigner:  xSigner,

		httpRPCEp: httpRPCEp,
	}, nil
}

// Refreshes the txs and utxos in case of extended disconnection/restarts.
// TODO: should be "primary.FetchState" again?
// here we assume there's no contending wallet user, so just cache everything...
func (w *refreshableWallet) refresh(httpRPCEp string) {
	// need updates when reconnected
	pClient := platformvm.NewClient(httpRPCEp)
	pw := p.NewWallet(w.pBuilder, w.pSigner, pClient, w.pBackend)

	// need updates when reconnected
	xClient := avm.NewClient(httpRPCEp, "X")
	xw := x.NewWallet(w.xBuilder, w.xSigner, xClient, w.xBackend)

	w.Wallet = primary.NewWallet(pw, xw)
	w.httpRPCEp = httpRPCEp
}

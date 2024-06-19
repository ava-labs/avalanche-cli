// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/rpc"
	subnetEvmUtils "github.com/ava-labs/subnet-evm/tests/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	BaseFeeFactor               = 2
	MaxPriorityFeePerGas        = 2500000000 // 2.5 gwei
	NativeTransferGas    uint64 = 21_000
	repeatsOnFailure            = 3
	sleepBetweenRepeats         = 1 * time.Second
)

func ContractAlreadyDeployed(
	client ethclient.Client,
	contractAddress string,
) (bool, error) {
	if bs, err := GetContractBytecode(client, contractAddress); err != nil {
		return false, err
	} else {
		return len(bs) != 0, nil
	}
}

func GetContractBytecode(
	client ethclient.Client,
	contractAddressStr string,
) ([]byte, error) {
	contractAddress := common.HexToAddress(contractAddressStr)
	var (
		code []byte
		err  error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		code, err = client.CodeAt(ctx, contractAddress, nil)
		if err == nil {
			break
		}
		err = fmt.Errorf(
			"failure obtaining code for %s on %#v: %w",
			contractAddressStr,
			client,
			err,
		)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return code, err
}

func GetAddressBalance(
	client ethclient.Client,
	addressStr string,
) (*big.Int, error) {
	address := common.HexToAddress(addressStr)
	var (
		balance *big.Int
		err     error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		balance, err = client.BalanceAt(ctx, address, nil)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure obtaining balance for %s on %#v: %w", addressStr, client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return balance, err
}

// Returns the gasFeeCap, gasTipCap, and nonce the be used when constructing a transaction from address
func CalculateTxParams(
	client ethclient.Client,
	addressStr string,
) (*big.Int, *big.Int, uint64, error) {
	baseFee, err := EstimateBaseFee(client)
	if err != nil {
		return nil, nil, 0, err
	}
	gasTipCap, err := SuggestGasTipCap(client)
	if err != nil {
		return nil, nil, 0, err
	}
	nonce, err := NonceAt(client, addressStr)
	if err != nil {
		return nil, nil, 0, err
	}
	gasFeeCap := baseFee.Mul(baseFee, big.NewInt(BaseFeeFactor))
	gasFeeCap.Add(gasFeeCap, big.NewInt(MaxPriorityFeePerGas))
	return gasFeeCap, gasTipCap, nonce, nil
}

func NonceAt(
	client ethclient.Client,
	addressStr string,
) (uint64, error) {
	address := common.HexToAddress(addressStr)
	var (
		nonce uint64
		err   error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		nonce, err = client.NonceAt(ctx, address, nil)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure obtaining nonce for %s on %#v: %w", addressStr, client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return nonce, err
}

func SuggestGasTipCap(
	client ethclient.Client,
) (*big.Int, error) {
	var (
		gasTipCap *big.Int
		err       error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		gasTipCap, err = client.SuggestGasTipCap(ctx)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure obtaining gas tip cap on %#v: %w", client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return gasTipCap, err
}

func EstimateBaseFee(
	client ethclient.Client,
) (*big.Int, error) {
	var (
		baseFee *big.Int
		err     error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		baseFee, err = client.EstimateBaseFee(ctx)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure estimating base fee on %#v: %w", client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return baseFee, err
}

func FundAddress(
	client ethclient.Client,
	sourceAddressPrivateKeyStr string,
	targetAddressStr string,
	amount *big.Int,
) error {
	sourceAddressPrivateKey, err := crypto.HexToECDSA(sourceAddressPrivateKeyStr)
	if err != nil {
		return err
	}
	sourceAddress := crypto.PubkeyToAddress(sourceAddressPrivateKey.PublicKey)
	gasFeeCap, gasTipCap, nonce, err := CalculateTxParams(client, sourceAddress.Hex())
	if err != nil {
		return err
	}
	targetAddress := common.HexToAddress(targetAddressStr)
	chainID, err := GetChainID(client)
	if err != nil {
		return err
	}
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &targetAddress,
		Gas:       NativeTransferGas,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Value:     amount,
	})
	txSigner := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, txSigner, sourceAddressPrivateKey)
	if err != nil {
		return err
	}
	if err := SendTransaction(client, signedTx); err != nil {
		return err
	}
	if _, b, err := WaitForTransaction(client, signedTx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure funding %s from %s amount %d", targetAddressStr, sourceAddress.Hex(), amount)
	}
	return nil
}

func IssueTx(
	client ethclient.Client,
	txStr string,
) error {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(common.FromHex(txStr)); err != nil {
		return err
	}
	if err := SendTransaction(client, tx); err != nil {
		return err
	}
	if receipt, b, err := WaitForTransaction(client, tx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure sending tx: got status %d expected %d", receipt.Status, types.ReceiptStatusSuccessful)
	}
	return nil
}

func SendTransaction(
	client ethclient.Client,
	tx *types.Transaction,
) error {
	var err error
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		err = client.SendTransaction(ctx, tx)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure sending transaction %#v to %#v: %w", tx, client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return err
}

func GetClient(rpcURL string) (ethclient.Client, error) {
	var (
		client ethclient.Client
		err    error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		client, err = ethclient.DialContext(ctx, rpcURL)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure connecting to %s: %w", rpcURL, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return client, err
}

func GetChainID(client ethclient.Client) (*big.Int, error) {
	var (
		chainID *big.Int
		err     error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		chainID, err = client.ChainID(ctx)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure getting chain id from client %#v: %w", client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return chainID, err
}

func GetTxOptsWithSigner(
	client ethclient.Client,
	prefundedPrivateKeyStr string,
) (*bind.TransactOpts, error) {
	prefundedPrivateKey, err := crypto.HexToECDSA(prefundedPrivateKeyStr)
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(client)
	if err != nil {
		return nil, fmt.Errorf("failure generating signer: %w", err)
	}
	return bind.NewKeyedTransactorWithChainID(prefundedPrivateKey, chainID)
}

func WaitForTransaction(
	client ethclient.Client,
	tx *types.Transaction,
) (*types.Receipt, bool, error) {
	var (
		err     error
		receipt *types.Receipt
		success bool
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		receipt, err = bind.WaitMined(ctx, client, tx)
		if err == nil {
			success = receipt.Status == types.ReceiptStatusSuccessful
			break
		}
		err = fmt.Errorf("failure waiting for tx %#v on client %#v: %w", tx, client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return receipt, success, err
}

// Returns the first log in 'logs' that is successfully parsed by 'parser'
func GetEventFromLogs[T any](logs []*types.Log, parser func(log types.Log) (T, error)) (T, error) {
	cumErrMsg := ""
	for i, log := range logs {
		event, err := parser(*log)
		if err == nil {
			return event, nil
		}
		if cumErrMsg != "" {
			cumErrMsg += "; "
		}
		cumErrMsg += fmt.Sprintf("log %d -> %s", i, err.Error())
	}
	return *new(T), fmt.Errorf("failed to find %T event in receipt logs: [%s]", *new(T), cumErrMsg)
}

func GetRPCClient(rpcURL string) (*rpc.Client, error) {
	var (
		client *rpc.Client
		err    error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		client, err = rpc.DialContext(ctx, rpcURL)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure connecting to rpc client on %s: %w", rpcURL, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return client, err
}

func DebugTraceTransaction(
	client *rpc.Client,
	txID string,
) (map[string]interface{}, error) {
	var (
		err   error
		trace map[string]interface{}
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		err = client.CallContext(
			ctx,
			&trace,
			"debug_traceTransaction",
			txID,
			map[string]string{"tracer": "callTracer"},
		)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure tracing tx %s for client %#v: %w", txID, client, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return trace, err
}

func GetTrace(rpcURL string, txID string) (map[string]interface{}, error) {
	client, err := GetRPCClient(rpcURL)
	if err != nil {
		return nil, err
	}
	return DebugTraceTransaction(client, txID)
}

func SetupProposerVM(
	endpoint string,
	privKeyStr string,
) error {
	privKey, err := crypto.HexToECDSA(privKeyStr)
	if err != nil {
		return err
	}
	client, err := GetClient(endpoint)
	if err != nil {
		return err
	}
	chainID, err := GetChainID(client)
	if err != nil {
		return err
	}
	return IssueTxsToActivateProposerVMFork(client, chainID, privKey)
}

func IssueTxsToActivateProposerVMFork(
	client ethclient.Client,
	chainID *big.Int,
	privKey *ecdsa.PrivateKey,
) error {
	var err error
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		err = subnetEvmUtils.IssueTxsToActivateProposerVMFork(ctx, chainID, privKey, client)
		if err == nil {
			break
		}
		err = fmt.Errorf(
			"failure issuing txs to activate proposer VM fork for client %#v: %w",
			client,
			err,
		)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return err
}

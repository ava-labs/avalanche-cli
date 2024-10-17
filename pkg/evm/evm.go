// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	avalancheWarp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/interfaces"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ava-labs/subnet-evm/predicate"
	"github.com/ava-labs/subnet-evm/rpc"
	subnetEvmUtils "github.com/ava-labs/subnet-evm/utils"
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

var UnknownErrorSelector = fmt.Errorf("unknown error selector")

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

func EstimateGasLimit(
	client ethclient.Client,
	msg interfaces.CallMsg,
) (uint64, error) {
	var (
		gasLimit uint64
		err      error
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		gasLimit, err = client.EstimateGas(ctx, msg)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure estimating gas limit on %#v: %w", client, err)
		time.Sleep(sleepBetweenRepeats)
	}
	return gasLimit, err
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

func GetSignedTxToMethodWithWarpMessage(
	client ethclient.Client,
	privateKeyStr string,
	warpMessage *avalancheWarp.Message,
	contract common.Address,
	callData []byte,
	value *big.Int,
) (*types.Transaction, error) {
	const defaultGasLimit = 2_000_000
	privateKey, err := crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		return nil, err
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	gasFeeCap, gasTipCap, nonce, err := CalculateTxParams(client, address.Hex())
	if err != nil {
		return nil, err
	}
	chainID, err := GetChainID(client)
	if err != nil {
		return nil, err
	}
	accessList := types.AccessList{
		types.AccessTuple{
			Address:     warp.ContractAddress,
			StorageKeys: subnetEvmUtils.BytesToHashSlice(predicate.PackPredicate(warpMessage.Bytes())),
		},
	}
	msg := interfaces.CallMsg{
		From:       address,
		To:         &contract,
		GasPrice:   nil,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Value:      value,
		Data:       callData,
		AccessList: accessList,
	}
	gasLimit, err := EstimateGasLimit(client, msg)
	if err != nil {
		// assuming this is related to the tx itself.
		// just using default gas limit, and let the user debug the
		// tx if needed so
		gasLimit = defaultGasLimit
	}
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    chainID,
		Nonce:      nonce,
		To:         &contract,
		Gas:        gasLimit,
		GasFeeCap:  gasFeeCap,
		GasTipCap:  gasTipCap,
		Value:      value,
		Data:       callData,
		AccessList: accessList,
	})
	txSigner := types.LatestSignerForChainID(chainID)
	return types.SignTx(tx, txSigner, privateKey)
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

func FindOutScheme(rpcURL string) (ethclient.Client, string, error) {
	if b, err := HasScheme(rpcURL); err != nil {
		return nil, "", err
	} else if b {
		return nil, "", fmt.Errorf("url does have scheme")
	}
	notDeterminedErr := fmt.Errorf("url %s has no scheme and protocol could not be determined", rpcURL)
	// let's start with ws it always give same error for http/https/wss
	scheme := "ws://"
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	client, err := ethclient.DialContext(ctx, scheme+rpcURL)
	if err == nil {
		return client, scheme, nil
	} else if !strings.Contains(err.Error(), "websocket: bad handshake") {
		return nil, "", notDeterminedErr
	}
	// wss give specific errors for http/http
	scheme = "wss://"
	client, err = ethclient.DialContext(ctx, scheme+rpcURL)
	if err == nil {
		return client, scheme, nil
	} else if !strings.Contains(err.Error(), "websocket: bad handshake") && // may be https
		!strings.Contains(err.Error(), "first record does not look like a TLS handshake") { // may be http
		return nil, "", notDeterminedErr
	}
	// https/http discrimination based on sending a specific query
	scheme = "https://"
	client, err = ethclient.DialContext(ctx, scheme+rpcURL)
	if err == nil {
		_, err = client.ChainID(ctx)
		switch {
		case err == nil:
			return client, scheme, nil
		case strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"):
			scheme = "http://"
			client, err = ethclient.DialContext(ctx, scheme+rpcURL)
			if err == nil {
				return client, scheme, nil
			}
		}
	}
	return nil, "", notDeterminedErr
}

func HasScheme(rpcURL string) (bool, error) {
	if parsedURL, err := url.Parse(rpcURL); err != nil {
		if !strings.Contains(err.Error(), "first path segment in URL cannot contain colon") {
			return false, err
		}
		return false, nil
	} else if parsedURL.Scheme == "" {
		return false, nil
	}
	return true, nil
}

func GetClient(rpcURL string) (ethclient.Client, error) {
	var (
		client ethclient.Client
		err    error
	)
	hasScheme, err := HasScheme(rpcURL)
	if err != nil {
		return nil, err
	}
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		if hasScheme {
			client, err = ethclient.DialContext(ctx, rpcURL)
		} else {
			client, _, err = FindOutScheme(rpcURL)
		}
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
	hasScheme, err := HasScheme(rpcURL)
	if err != nil {
		return nil, err
	}
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		if !hasScheme {
			_, scheme, findErr := FindOutScheme(rpcURL)
			if findErr == nil {
				client, err = rpc.DialContext(ctx, scheme+rpcURL)
			} else {
				err = findErr
			}
		} else {
			client, err = rpc.DialContext(ctx, rpcURL)
		}
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
		time.Sleep(sleepBetweenRepeats)
	}
	return trace, err
}

func DebugTraceCall(
	client *rpc.Client,
	toTrace map[string]string,
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
			"debug_traceCall",
			toTrace,
			"latest",
			map[string]interface{}{
				"tracer": "callTracer",
				"tracerConfig": map[string]interface{}{
					"onlyTopCall": false,
				},
			},
		)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure tracing call for client %#v: %w", client, err)
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
		err = issueTxsToActivateProposerVMFork(client, ctx, chainID, privKey)
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

// issueTxsToActivateProposerVMFork issues transactions at the current
// timestamp, which should be after the ProposerVM activation time (aka
// ApricotPhase4). This should generate a PostForkBlock because its parent block
// (genesis) has a timestamp (0) that is greater than or equal to the fork
// activation time of 0. Therefore, subsequent blocks should be built with
// BuildBlockWithContext.
func issueTxsToActivateProposerVMFork(
	client ethclient.Client,
	ctx context.Context,
	chainID *big.Int,
	fundedKey *ecdsa.PrivateKey,
) error {
	const numTriggerTxs = 2 // Number of txs needed to activate the proposer VM fork
	addr := crypto.PubkeyToAddress(fundedKey.PublicKey)
	gasPrice := big.NewInt(params.MinGasPrice)
	txSigner := types.LatestSignerForChainID(chainID)
	for i := 0; i < numTriggerTxs; i++ {
		prevBlockNumber, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}
		nonce, err := client.NonceAt(ctx, addr, nil)
		if err != nil {
			return err
		}
		tx := types.NewTransaction(
			nonce, addr, common.Big1, params.TxGas, gasPrice, nil)
		triggerTx, err := types.SignTx(tx, txSigner, fundedKey)
		if err != nil {
			return err
		}
		if err := client.SendTransaction(ctx, triggerTx); err != nil {
			return err
		}
		if err := WaitForNewBlock(client, ctx, prevBlockNumber, 0, 0); err != nil {
			return err
		}
	}
	return nil
}

func WaitForNewBlock(
	client ethclient.Client,
	ctx context.Context,
	prevBlockNumber uint64,
	totalDuration time.Duration,
	stepDuration time.Duration,
) error {
	if stepDuration == 0 {
		stepDuration = 1 * time.Second
	}
	if totalDuration == 0 {
		totalDuration = 10 * time.Second
	}
	steps := totalDuration / stepDuration
	for seconds := 0; seconds < int(steps); seconds++ {
		blockNumber, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if blockNumber > prevBlockNumber {
			return nil
		}
		time.Sleep(stepDuration)
	}
	return fmt.Errorf("new block not produced in %f seconds", totalDuration.Seconds())
}

func ExtractWarpMessageFromReceipt(
	client ethclient.Client,
	ctx context.Context,
	receipt *types.Receipt,
) (*avalancheWarp.UnsignedMessage, error) {
	logs, err := client.FilterLogs(ctx, interfaces.FilterQuery{
		BlockHash: &receipt.BlockHash,
		Addresses: []common.Address{warp.Module.Address},
	})
	if err != nil {
		return nil, err
	}
	if len(logs) != 1 {
		return nil, fmt.Errorf("expected block to contain 1 warp log, got %d", len(logs))
	}
	txLog := logs[0]
	return warp.UnpackSendWarpEventDataToMessage(txLog.Data)
}

func GetFunctionSelector(functionSignature string) string {
	return "0x" + hex.EncodeToString(crypto.Keccak256([]byte(functionSignature))[:4])
}

func GetErrorFromTrace(
	trace map[string]interface{},
	functionSignatureToError map[string]error,
) (error, error) {
	traceOutputI, ok := trace["output"]
	if !ok {
		return nil, fmt.Errorf("trace does not contain output field")
	}
	traceOutput, ok := traceOutputI.(string)
	if !ok {
		return nil, fmt.Errorf("expected type string for trace output, got %T", traceOutputI)
	}
	traceOutputBytes, err := hex.DecodeString(strings.TrimPrefix(traceOutput, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failure decoding trace output: %w", err)
	}
	if len(traceOutputBytes) < 4 {
		return nil, fmt.Errorf("less than 4 bytes in trace output")
	}
	traceErrorSelector := "0x" + hex.EncodeToString(traceOutputBytes[:4])
	for errorSignature, err := range functionSignatureToError {
		errorSelector := GetFunctionSelector(errorSignature)
		if traceErrorSelector == errorSelector {
			return err, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", UnknownErrorSelector, traceErrorSelector)
}

func TransactionError(tx *types.Transaction, err error, msg string, args ...interface{}) error {
	msgSuffix := ": %w"
	if tx != nil {
		msgSuffix += fmt.Sprintf(" (txHash=%s)", tx.Hash().String())
	} else {
		msgSuffix += " (tx failed to be submitted)"
	}
	args = append(args, err)
	return fmt.Errorf(msg+msgSuffix, args...)
}

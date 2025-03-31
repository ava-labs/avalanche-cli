// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
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

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
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
	repeatsOnFailure            = 3
	sleepBetweenRepeats         = 1 * time.Second
	baseFeeFactor               = 2
	maxPriorityFeePerGas        = 2500000000 // 2.5 gwei
	nativeTransferGas    uint64 = 21_000
)

var ErrUnknownErrorSelector = fmt.Errorf("unknown error selector")

// wraps over ethclient for calls used by SDK. includes:
// - finds out url scheme in case it is missing, to connect to ws/wss/http/https
// - repeats to try to recover from failures, generating its own context for each call
// - logs rpc url in case of failure
// - receives addresses and private keys as strings
type Client struct {
	EthClient ethclient.Client
	URL       string
}

// indicates if the given rpc url has schema or not
func HasScheme(rpcURL string) (bool, error) {
	if parsedURL, err := url.Parse(rpcURL); err != nil {
		if !strings.Contains(err.Error(), "first path segment in URL cannot contain colon") {
			return false, err
		}
		return false, nil
	} else {
		return parsedURL.Scheme != "", nil
	}
}

// tries to connect an ethclient to a rpc url without scheme,
// by trying out different possible schemes: ws, wss, https, http
func GetClientWithoutScheme(rpcURL string) (ethclient.Client, string, error) {
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

// connects an evm client to the given [rpcURL]
// supports [repeatsOnFailure] failures
func GetClient(rpcURL string) (Client, error) {
	client := Client{
		URL: rpcURL,
	}
	hasScheme, err := HasScheme(rpcURL)
	if err != nil {
		return client, fmt.Errorf("failure determining the scheme of url %s: %w", rpcURL, err)
	}
	client.EthClient, err = utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (ethclient.Client, error) {
			if hasScheme {
				return ethclient.DialContext(ctx, rpcURL)
			} else {
				client, _, err := GetClientWithoutScheme(rpcURL)
				return client, err
			}
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure connecting to %s: %w", rpcURL, err)
	}
	return client, err
}

// closes underlying ethclient connection
func (client Client) Close() {
	client.EthClient.Close()
}

// indicates wether a contract is deployed on [contractAddress]
// supports [repeatsOnFailure] failures
func (client Client) ContractAlreadyDeployed(
	contractAddress string,
) (bool, error) {
	if bs, err := client.GetContractBytecode(contractAddress); err != nil {
		return false, err
	} else {
		return len(bs) != 0, nil
	}
}

// returns the contract bytecode at [contractAddress]
// supports [repeatsOnFailure] failures
func (client Client) GetContractBytecode(
	contractAddressStr string,
) ([]byte, error) {
	contractAddress := common.HexToAddress(contractAddressStr)
	code, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) ([]byte, error) {
			return client.EthClient.CodeAt(ctx, contractAddress, nil)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf(
			"failure obtaining code from %s at address %s: %w",
			client.URL,
			contractAddressStr,
			err,
		)
	}
	return code, err
}

// returns the balance for [privateKey]
// supports [repeatsOnFailure] failures
func (client Client) GetPrivateKeyBalance(
	privateKey string,
) (*big.Int, error) {
	addr, err := PrivateKeyToAddress(privateKey)
	if err != nil {
		return nil, err
	}
	return client.GetAddressBalance(addr.Hex())
}

// returns the balance for [address]
// supports [repeatsOnFailure] failures
func (client Client) GetAddressBalance(
	addressStr string,
) (*big.Int, error) {
	address := common.HexToAddress(addressStr)
	balance, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*big.Int, error) {
			return client.EthClient.BalanceAt(ctx, address, nil)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure obtaining balance for %s on %s: %w", addressStr, client.URL, err)
	}
	return balance, err
}

// returns the nonce at [address]
// supports [repeatsOnFailure] failures
func (client Client) NonceAt(
	addressStr string,
) (uint64, error) {
	address := common.HexToAddress(addressStr)
	nonce, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (uint64, error) {
			return client.EthClient.NonceAt(ctx, address, nil)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure obtaining nonce for %s on %s: %w", addressStr, client.URL, err)
	}
	return nonce, err
}

// returns the suggested gas tip
// supports [repeatsOnFailure] failures
func (client Client) SuggestGasTipCap() (*big.Int, error) {
	gasTipCap, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*big.Int, error) {
			return client.EthClient.SuggestGasTipCap(ctx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure obtaining gas tip cap on %s: %w", client.URL, err)
	}
	return gasTipCap, err
}

// returns the estimated base fee
// supports [repeatsOnFailure] failures
func (client Client) EstimateBaseFee() (*big.Int, error) {
	baseFee, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*big.Int, error) {
			return client.EthClient.EstimateBaseFee(ctx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure estimating base fee on %s: %w", client.URL, err)
	}
	return baseFee, err
}

// Returns gasFeeCap, gasTipCap, and nonce to be used when constructing a transaction
// supports [repeatsOnFailure] failures on each step
func (client Client) CalculateTxParams(
	address string,
) (*big.Int, *big.Int, uint64, error) {
	baseFee, err := client.EstimateBaseFee()
	if err != nil {
		return nil, nil, 0, err
	}
	gasTipCap, err := client.SuggestGasTipCap()
	if err != nil {
		return nil, nil, 0, err
	}
	nonce, err := client.NonceAt(address)
	if err != nil {
		return nil, nil, 0, err
	}
	gasFeeCap := baseFee.Mul(baseFee, big.NewInt(baseFeeFactor))
	gasFeeCap.Add(gasFeeCap, big.NewInt(maxPriorityFeePerGas))
	return gasFeeCap, gasTipCap, nonce, nil
}

// returns the estimated gas limit
// supports [repeatsOnFailure] failures
func (client Client) EstimateGasLimit(
	msg interfaces.CallMsg,
) (uint64, error) {
	gasLimit, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (uint64, error) {
			return client.EthClient.EstimateGas(ctx, msg)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure estimating gas limit on %s: %w", client.URL, err)
	}
	return gasLimit, err
}

// returns the chain ID
// supports [repeatsOnFailure] failures
func (client Client) GetChainID() (*big.Int, error) {
	chainID, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*big.Int, error) {
			return client.EthClient.ChainID(ctx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure getting chain id from %s: %w", client.URL, err)
	}
	return chainID, err
}

// sends [tx]
// supports [repeatsOnFailure] failures
func (client Client) SendTransaction(
	tx *types.Transaction,
) error {
	_, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (any, error) {
			return nil, client.EthClient.SendTransaction(ctx, tx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure sending transaction %#v to %s: %w", tx, client.URL, err)
	}
	return err
}

// waits for [tx]'s receipt to have successful state
// supports [repeatsOnFailure] failures
func (client Client) WaitForTransaction(
	tx *types.Transaction,
) (*types.Receipt, bool, error) {
	receipt, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*types.Receipt, error) {
			return bind.WaitMined(ctx, client.EthClient, tx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure waiting for tx %#v on %s: %w", tx, client.URL, err)
	}
	var success bool
	if receipt != nil {
		success = receipt.Status == types.ReceiptStatusSuccessful
	}
	return receipt, success, err
}

// transfers [amount] to [targetAddressStr] using [sourceAddressPrivateKeyStr]
// supports [repeatsOnFailure] failures on each step
func (client Client) FundAddress(
	sourceAddressPrivateKeyStr string,
	targetAddressStr string,
	amount *big.Int,
) error {
	sourceAddressPrivateKey, err := crypto.HexToECDSA(sourceAddressPrivateKeyStr)
	if err != nil {
		return err
	}
	sourceAddress := crypto.PubkeyToAddress(sourceAddressPrivateKey.PublicKey)
	gasFeeCap, gasTipCap, nonce, err := client.CalculateTxParams(sourceAddress.Hex())
	if err != nil {
		return err
	}
	targetAddress := common.HexToAddress(targetAddressStr)
	chainID, err := client.GetChainID()
	if err != nil {
		return err
	}
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &targetAddress,
		Gas:       nativeTransferGas,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Value:     amount,
	})
	txSigner := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, txSigner, sourceAddressPrivateKey)
	if err != nil {
		return err
	}
	if err := client.SendTransaction(signedTx); err != nil {
		return err
	}
	if _, b, err := client.WaitForTransaction(signedTx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure funding %s from %s amount %d", targetAddressStr, sourceAddress.Hex(), amount)
	}
	return nil
}

// encode [txStr] to binary, sends and waits for it
// supports [repeatsOnFailure] failures on each step
func (client Client) IssueTx(
	txStr string,
) error {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(common.FromHex(txStr)); err != nil {
		return err
	}
	if err := client.SendTransaction(tx); err != nil {
		return err
	}
	if receipt, b, err := client.WaitForTransaction(tx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure sending tx: got status %d expected %d", receipt.Status, types.ReceiptStatusSuccessful)
	}
	return nil
}

// returns tx options that include signer for [prefundedPrivateKeyStr]
// supports [repeatsOnFailure] failures when gathering chain info
func (client Client) GetTxOptsWithSigner(
	prefundedPrivateKeyStr string,
) (*bind.TransactOpts, error) {
	prefundedPrivateKey, err := crypto.HexToECDSA(prefundedPrivateKeyStr)
	if err != nil {
		return nil, err
	}
	chainID, err := client.GetChainID()
	if err != nil {
		return nil, fmt.Errorf("failure generating signer: %w", err)
	}
	return bind.NewKeyedTransactorWithChainID(prefundedPrivateKey, chainID)
}

// waits for [timeout] until evm is bootstrapped
// considers evm is bootstrapped if it responds to an evm call (ChainID)
func (client Client) WaitForEVMBootstrapped(timeout time.Duration) error {
	const stepDuration = 5 * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	startTime := time.Now()
	for {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		if _, err := client.EthClient.ChainID(ctx); err == nil {
			return nil
		} else {
			if time.Since(startTime) > timeout {
				return fmt.Errorf("client at %s not bootstrapped after %.2f seconds: %w", client.URL, timeout.Seconds(), err)
			}
			time.Sleep(stepDuration)
		}
	}
}

func GetTxToMethodWithWarpMessage(
	client Client,
	generateRawTxOnly bool,
	from common.Address,
	privateKeyStr string,
	warpMessage *avalancheWarp.Message,
	contract common.Address,
	callData []byte,
	value *big.Int,
) (*types.Transaction, error) {
	const defaultGasLimit = 2_000_000
	var (
		privateKey *ecdsa.PrivateKey
		err        error
	)
	if privateKeyStr == "" && from == (common.Address{}) {
		return nil, fmt.Errorf("from address and private key can't be both empty at GetTxToMethodWithWarpMessage")
	}
	if !generateRawTxOnly && privateKeyStr == "" {
		return nil, fmt.Errorf("from private key must be defined to be able to sign the tx at GetTxToMethodWithWarpMessage")
	}
	if privateKeyStr != "" {
		privateKey, err = crypto.HexToECDSA(privateKeyStr)
		if err != nil {
			return nil, err
		}
		if from == (common.Address{}) {
			from = crypto.PubkeyToAddress(privateKey.PublicKey)
		}
	}
	gasFeeCap, gasTipCap, nonce, err := client.CalculateTxParams(from.Hex())
	if err != nil {
		return nil, err
	}
	chainID, err := client.GetChainID()
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
		From:       from,
		To:         &contract,
		GasPrice:   nil,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Value:      value,
		Data:       callData,
		AccessList: accessList,
	}
	gasLimit, err := client.EstimateGasLimit(msg)
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
	if generateRawTxOnly {
		return tx, nil
	}
	txSigner := types.LatestSignerForChainID(chainID)
	return types.SignTx(tx, txSigner, privateKey)
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
			_, scheme, findErr := GetClientWithoutScheme(rpcURL)
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
	chainID, err := client.GetChainID()
	if err != nil {
		return err
	}
	_, err = utils.Retry(
		func() (any, error) {
			return nil, issueTxsToActivateProposerVMFork(client, chainID, privKey)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure issuing tx to activate proposer VM: %w", err)
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
	client Client,
	chainID *big.Int,
	fundedKey *ecdsa.PrivateKey,
) error {
	const numTriggerTxs = 2 // Number of txs needed to activate the proposer VM fork
	addr := crypto.PubkeyToAddress(fundedKey.PublicKey)
	gasPrice := big.NewInt(params.MinGasPrice)
	txSigner := types.LatestSignerForChainID(chainID)
	for i := 0; i < numTriggerTxs; i++ {
		ctx, cancel := utils.GetTimedContext(1 * time.Minute)
		defer cancel()
		prevBlockNumber, err := client.EthClient.BlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("client.BlockNumber failure at step %d: %w", i, err)
		}
		nonce, err := client.EthClient.NonceAt(ctx, addr, nil)
		if err != nil {
			return fmt.Errorf("client.NonceAt failure at step %d: %w", i, err)
		}
		tx := types.NewTransaction(nonce, addr, common.Big1, params.TxGas, gasPrice, nil)
		triggerTx, err := types.SignTx(tx, txSigner, fundedKey)
		if err != nil {
			return fmt.Errorf("types.SignTx failure at step %d: %w", i, err)
		}
		if err := client.EthClient.SendTransaction(ctx, triggerTx); err != nil {
			return fmt.Errorf("client.SendTransaction failure at step %d: %w", i, err)
		}
		if err := WaitForNewBlock(client, ctx, prevBlockNumber, 0, 0); err != nil {
			return fmt.Errorf("WaitForNewBlock failure at step %d: %w", i, err)
		}
	}
	return nil
}

func WaitForNewBlock(
	client Client,
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
	for step := 0; step < int(steps); step++ {
		blockNumber, err := client.EthClient.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if blockNumber > prevBlockNumber {
			return nil
		}
		time.Sleep(stepDuration)
	}
	return fmt.Errorf("no new block produced on %s in %f seconds", client.URL, totalDuration.Seconds())
}

func ExtractWarpMessageFromReceipt(
	client Client,
	ctx context.Context,
	receipt *types.Receipt,
) (*avalancheWarp.UnsignedMessage, error) {
	logs, err := client.EthClient.FilterLogs(ctx, interfaces.FilterQuery{
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
	return nil, fmt.Errorf("%w: %s", ErrUnknownErrorSelector, traceErrorSelector)
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

func TxDump(description string, tx *types.Transaction) error {
	bs, err := tx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failure marshalling raw evm tx: %w", err)
	}
	ux.Logger.PrintToUser("Tx Dump For %s:", description)
	ux.Logger.PrintToUser("0x%s", hex.EncodeToString(bs))
	ux.Logger.PrintToUser("Calldata Dump:")
	ux.Logger.PrintToUser("0x%s", hex.EncodeToString(tx.Data()))
	if len(tx.AccessList()) > 0 {
		ux.Logger.PrintToUser("Access List Dump:")
		for _, t := range tx.AccessList() {
			ux.Logger.PrintToUser("  Address: %s", t.Address)
			for _, s := range t.StorageKeys {
				ux.Logger.PrintToUser("  Storage: %s", s)
			}
		}
	}
	return nil
}

func PrivateKeyToAddress(privateKey string) (common.Address, error) {
	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(pk.PublicKey), nil
}

func (client Client) BlockNumber(ctx context.Context) (uint64, error) {
	return client.EthClient.BlockNumber(ctx)
}

func (client Client) BlockByNumber(ctx context.Context, n *big.Int) (*types.Block, error) {
	return client.EthClient.BlockByNumber(ctx, n)
}

func (client Client) FilterLogs(ctx context.Context, query interfaces.FilterQuery) ([]types.Log, error) {
	return client.EthClient.FilterLogs(ctx, query)
}

func (client Client) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	return client.EthClient.TransactionReceipt(ctx, hash)
}

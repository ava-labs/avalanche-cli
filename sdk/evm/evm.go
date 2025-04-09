// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/utils"
	avalancheWarp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/interfaces"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ava-labs/subnet-evm/predicate"
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

// used to mock the connection function
var ethclientDialContext = ethclient.DialContext

// wraps over ethclient for calls used by SDK. featues:
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
		return strings.Contains(rpcURL, "://") && parsedURL.Scheme != "", nil
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
	client, err := ethclientDialContext(ctx, scheme+rpcURL)
	if err == nil {
		return client, scheme, nil
	} else if !strings.Contains(err.Error(), "websocket: bad handshake") {
		return nil, "", notDeterminedErr
	}
	// wss give specific errors for http/http
	scheme = "wss://"
	client, err = ethclientDialContext(ctx, scheme+rpcURL)
	if err == nil {
		return client, scheme, nil
	} else if !strings.Contains(err.Error(), "websocket: bad handshake") && // may be https
		!strings.Contains(err.Error(), "first record does not look like a TLS handshake") { // may be http
		return nil, "", notDeterminedErr
	}
	// https/http discrimination based on sending a specific query
	scheme = "https://"
	client, err = ethclientDialContext(ctx, scheme+rpcURL)
	if err == nil {
		_, err = client.ChainID(ctx)
		switch {
		case err == nil:
			return client, scheme, nil
		case strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"):
			scheme = "http://"
			client, err = ethclientDialContext(ctx, scheme+rpcURL)
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
				return ethclientDialContext(ctx, rpcURL)
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

// generates a transaction signed with [privateKeyStr], calling a [contract] method using [callData]
// including [warpMessage] in the tx accesslist
// if [generateRawTxOnly] is set, it generates a similar, unsigned tx, with given [from] address
func (client Client) TransactWithWarpMessage(
	from common.Address,
	privateKeyStr string,
	warpMessage *avalancheWarp.Message,
	contract common.Address,
	callData []byte,
	value *big.Int,
	generateRawTxOnly bool,
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

// gets block [n]
// supports [repeatsOnFailure] failures
func (client Client) BlockByNumber(n *big.Int) (*types.Block, error) {
	block, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*types.Block, error) {
			return client.EthClient.BlockByNumber(ctx, n)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure retrieving block %d on %s: %w", n, client.URL, err)
	}
	return block, err
}

// get logs as given by [query]
// supports [repeatsOnFailure] failures
func (client Client) FilterLogs(query interfaces.FilterQuery) ([]types.Log, error) {
	logs, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) ([]types.Log, error) {
			return client.EthClient.FilterLogs(ctx, query)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure retrieving logs on %s: %w", client.URL, err)
	}
	return logs, err
}

// get tx receipt for [hash]
// supports [repeatsOnFailure] failures
func (client Client) TransactionReceipt(hash common.Hash) (*types.Receipt, error) {
	receipt, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*types.Receipt, error) {
			return client.EthClient.TransactionReceipt(ctx, hash)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure retrieving receipt for %s on %s: %w", hash, client.URL, err)
	}
	return receipt, err
}

// gets current height
// supports [repeatsOnFailure] failures
func (client Client) BlockNumber() (uint64, error) {
	blockNumber, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (uint64, error) {
			return client.EthClient.BlockNumber(ctx)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure retrieving height (block number) on %s: %w", client.URL, err)
	}
	return blockNumber, err
}

// waits until current height is bigger than the given previous height at [prevBlockNumber]
// supports [repeatsOnFailure] failures on each step
func (client Client) WaitForNewBlock(
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
		blockNumber, err := client.BlockNumber()
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

// issue dummy txs to create the given number of blocks
func (client Client) CreateDummyBlocks(
	numBlocks int,
	privKeyStr string,
) error {
	addr, err := PrivateKeyToAddress(privKeyStr)
	if err != nil {
		return err
	}
	privKey, err := crypto.HexToECDSA(privKeyStr)
	if err != nil {
		return err
	}
	chainID, err := client.GetChainID()
	if err != nil {
		return err
	}
	gasPrice := big.NewInt(params.MinGasPrice)
	txSigner := types.LatestSignerForChainID(chainID)
	for i := 0; i < numBlocks; i++ {
		prevBlockNumber, err := client.BlockNumber()
		if err != nil {
			return fmt.Errorf("client.BlockNumber failure at step %d: %w", i, err)
		}
		nonce, err := client.NonceAt(addr.Hex())
		if err != nil {
			return fmt.Errorf("client.NonceAt failure at step %d: %w", i, err)
		}
		// send Big1 to himself
		tx := types.NewTransaction(nonce, addr, common.Big1, params.TxGas, gasPrice, nil)
		triggerTx, err := types.SignTx(tx, txSigner, privKey)
		if err != nil {
			return fmt.Errorf("types.SignTx failure at step %d: %w", i, err)
		}
		if err := client.SendTransaction(triggerTx); err != nil {
			return fmt.Errorf("client.SendTransaction failure at step %d: %w", i, err)
		}
		if err := client.WaitForNewBlock(prevBlockNumber, 0, 0); err != nil {
			return fmt.Errorf("WaitForNewBlock failure at step %d: %w", i, err)
		}
	}
	return nil
}

// issue transactions on [client] so as to activate Proposer VM Fork
// this should generate a PostForkBlock because its parent block
// (genesis) has a timestamp (0) that is greater than or equal to the fork
// activation time of 0. Therefore, subsequent blocks should be built with
// BuildBlockWithContext.
// the current timestamp should be after the ProposerVM activation time (aka ApricotPhase4).
// supports [repeatsOnFailure] failures on each step
func (client Client) SetupProposerVM(
	privKey string,
) error {
	const numBlocks = 2 // Number of blocks needed to activate the proposer VM fork
	_, err := utils.Retry(
		func() (any, error) {
			return nil, client.CreateDummyBlocks(numBlocks, privKey)
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure issuing tx to activate proposer VM: %w", err)
	}
	return err
}

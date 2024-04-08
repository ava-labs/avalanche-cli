// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/rpc"
	subnetEvmUtils "github.com/ava-labs/subnet-evm/tests/utils"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ethereum/go-ethereum/common"
)

const (
	BaseFeeFactor               = 2
	MaxPriorityFeePerGas        = 2500000000 // 2.5 gwei
	NativeTransferGas    uint64 = 21_000
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
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	contractAddress := common.HexToAddress(contractAddressStr)
	return client.CodeAt(ctx, contractAddress, nil)
}

func GetAddressBalance(
	client ethclient.Client,
	addressStr string,
) (*big.Int, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	address := common.HexToAddress(addressStr)
	return client.BalanceAt(ctx, address, nil)
}

// Returns the gasFeeCap, gasTipCap, and nonce the be used when constructing a transaction from address
func CalculateTxParams(
	client ethclient.Client,
	addressStr string,
) (*big.Int, *big.Int, uint64, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	address := common.HexToAddress(addressStr)
	baseFee, err := client.EstimateBaseFee(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	gasTipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	nonce, err := client.NonceAt(ctx, address, nil)
	if err != nil {
		return nil, nil, 0, err
	}
	gasFeeCap := baseFee.Mul(baseFee, big.NewInt(BaseFeeFactor))
	gasFeeCap.Add(gasFeeCap, big.NewInt(MaxPriorityFeePerGas))
	return gasFeeCap, gasTipCap, nonce, nil
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
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	chainID, err := client.ChainID(ctx)
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
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return err
	}
	if _, b, err := WaitForTransaction(client, signedTx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure funding %s from %s amount %d", targetAddressStr, sourceAddress.Hex(), amount)
	}
	return nil
}

func ActivateProposerVM(
	client ethclient.Client,
	privateKeyStr string,
) error {
	privateKey, err := crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	return subnetEvmUtils.IssueTxsToActivateProposerVMFork(
		ctx,
		chainID,
		privateKey,
		client,
	)
}

func IssueTx(
	client ethclient.Client,
	txStr string,
) error {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(common.FromHex(txStr)); err != nil {
		return err
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	if err := client.SendTransaction(ctx, tx); err != nil {
		return err
	}
	if _, b, err := WaitForTransaction(client, tx); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("failure sending tx")
	}
	return nil
}

func GetClient(rpcURL string) (ethclient.Client, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return ethclient.DialContext(ctx, rpcURL)
}

func GetTxOptsWithSigner(client ethclient.Client, prefundedPrivateKeyStr string) (*bind.TransactOpts, error) {
	prefundedPrivateKey, err := crypto.HexToECDSA(prefundedPrivateKeyStr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(prefundedPrivateKey, chainID)
}

func WaitForTransaction(
	client ethclient.Client,
	tx *types.Transaction,
) (*types.Receipt, bool, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, false, err
	}

	return receipt, receipt.Status == types.ReceiptStatusSuccessful, nil
}

// Returns the first log in 'logs' that is successfully parsed by 'parser'
func GetEventFromLogs[T any](logs []*types.Log, parser func(log types.Log) (T, error)) (T, error) {
	for _, log := range logs {
		event, err := parser(*log)
		if err == nil {
			return event, nil
		}
	}
	return *new(T), fmt.Errorf("failed to find %T event in receipt logs", *new(T))
}

func GetTrace(rpcURL string, txID string) (map[string]interface{}, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	client, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = client.CallContext(ctx, &result, "debug_traceTransaction", txID, map[string]string{"tracer": "callTracer"})
	return result, err
}

func SetupProposerVM(
	endpoint string,
	privKeyStr string,
) error {
	client, err := GetClient(endpoint)
	if err != nil {
		return fmt.Errorf("failure connecting to %s: %w", endpoint, err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	privKey, err := crypto.HexToECDSA(privKeyStr)
	if err != nil {
		return err
	}
	return subnetEvmUtils.IssueTxsToActivateProposerVMFork(ctx, chainID, privKey, client)
}

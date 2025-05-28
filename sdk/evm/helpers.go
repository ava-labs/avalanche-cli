// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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

// transform a tx operation error into an error that contains:
// - the [err] itself
// - the [tx] hash (or information on the tx not being submitted)
// - another descriptive [msg], together with formated [args]
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

// dumps a [tx] hexa description, for it to be separately issued using external tools
func TxDump(description string, tx *types.Transaction) (string, error) {
	if tx == nil {
		return "", fmt.Errorf("can't dump nil tx")
	}
	bs, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failure marshalling raw evm tx: %w", err)
	}
	txDump := ""
	txDump += fmt.Sprintf("Tx Dump For %s:\n", description)
	txDump += fmt.Sprintf("0x%s\n", hex.EncodeToString(bs))
	txDump += "Calldata Dump:\n"
	txDump += fmt.Sprintf("0x%s\n", hex.EncodeToString(tx.Data()))
	if len(tx.AccessList()) > 0 {
		txDump += "Access List Dump:\n"
		for _, t := range tx.AccessList() {
			txDump += fmt.Sprintf("  Address: %s\n", t.Address)
			for _, s := range t.StorageKeys {
				txDump += fmt.Sprintf("  Storage: %s\n", s)
			}
		}
	}
	return txDump, nil
}

// returns the public address associated with [privateKey]
func PrivateKeyToAddress(privateKey string) (common.Address, error) {
	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(pk.PublicKey), nil
}

// ConvertToNanoAvax converts a balance in Avax to NanoAvax.
// It adds 0.5 to the balance before dividing by 1e9 to round
// it to the nearest whole number.
func ConvertToNanoAvax(balance *big.Int) *big.Int {
	divisor := big.NewInt(int64(units.Avax))
	half := new(big.Int).Div(divisor, big.NewInt(2))
	adjusted := new(big.Int).Add(balance, half)
	return new(big.Int).Div(adjusted, divisor)
}

func CalculateEvmFeeInAvax(gasUsed uint64, gasPrice *big.Int) float64 {
	gasUsedBig := new(big.Int).SetUint64(gasUsed)
	totalCost := new(big.Int).Mul(gasUsedBig, gasPrice)

	totalCostInNanoAvax := ConvertToNanoAvax(totalCost)

	result, _ := new(big.Float).SetInt(totalCostInNanoAvax).Float64()
	return result / float64(units.Avax)
}

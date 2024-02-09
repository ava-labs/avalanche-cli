// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

func ContractAlreadyDeployed(rpcURL string, contractAddress string) (bool, error) {
	bytecode, err := GetContractBytecode(rpcURL, contractAddress)
	if err != nil {
		return false, err
	}
	return bytecode != "0x", nil
}

func GetContractBytecode(rpcURL string, contractAddress string) (string, error) {
	// TODO: don't use forge for this
	cmd := exec.Command(
		"cast",
		"code",
		"--rpc-url",
		rpcURL,
		contractAddress,
	)
	outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		if outBuf.String() != "" {
			fmt.Println(outBuf.String())
		}
		if errBuf.String() != "" {
			fmt.Println(errBuf.String())
		}
		return "", fmt.Errorf("couldn't get contract %s bytecode from rpc %s: %w", contractAddress, rpcURL, err)
	}
	if errBuf.String() != "" {
		fmt.Println(errBuf.String())
	}
	return strings.TrimSpace(outBuf.String()), nil
}

func GetAddressBalance(rpcURL string, address string) (uint64, error) {
	// TODO: don't use forge for this
	cmd := exec.Command(
		"cast",
		"balance",
		"--rpc-url",
		rpcURL,
		address,
	)
	outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		if outBuf.String() != "" {
			fmt.Println(outBuf.String())
		}
		if errBuf.String() != "" {
			fmt.Println(errBuf.String())
		}
		return 0, fmt.Errorf("couldn't get address %s balance from rpc %s: %w", address, rpcURL, err)
	}
	if errBuf.String() != "" {
		fmt.Println(errBuf.String())
	}
	balanceStr := strings.TrimSpace(outBuf.String())
	balance, err := strconv.ParseUint(balanceStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse address %s balance %s from rpc %s: %w", address, balanceStr, rpcURL, err)
	}
	return balance, nil
}

func FundAddress(rpcURL string, sourceAddressPrivateKey string, targetAddress string, amount uint64) error {
	// TODO: don't use forge for this
	cmd := exec.Command(
		"cast",
		"send",
		"--json",
		"--rpc-url",
		rpcURL,
		"--private-key",
		sourceAddressPrivateKey,
		"--value",
		fmt.Sprint(amount),
		targetAddress,
	)
	outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		if outBuf.String() != "" {
			fmt.Println(outBuf.String())
		}
		if errBuf.String() != "" {
			fmt.Println(errBuf.String())
		}
		return fmt.Errorf("couldn't fund address %s balance from rpc %s: %w", targetAddress, rpcURL, err)
	}
	if errBuf.String() != "" {
		fmt.Println(errBuf.String())
	}
	return checkStatus("evm.FundAddress", outBuf.String())
}

func IssueTx(rpcURL string, tx string) error {
	// TODO: don't use forge for this
	cmd := exec.Command(
		"cast",
		"publish",
		"--rpc-url",
		rpcURL,
		tx,
	)
	outBuf, errBuf := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		if outBuf.String() != "" {
			fmt.Println(outBuf.String())
		}
		if errBuf.String() != "" {
			fmt.Println(errBuf.String())
		}
		return fmt.Errorf("couldn't issue tx into rpc %s: %w", rpcURL, err)
	}
	if errBuf.String() != "" {
		fmt.Println(errBuf.String())
	}
	return checkStatus("evm.IssueTx", outBuf.String())
}

func checkStatus(title string, jsonOutput string) error {
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &jsonMap); err != nil {
		return fmt.Errorf("%s: could not parse json output %s: %w", title, jsonOutput, err)
	}
	statusI, ok := jsonMap["status"]
	if !ok {
		return fmt.Errorf("%s: status field not found on json response %s", title, jsonOutput)
	}
	status, ok := statusI.(string)
	if !ok {
		return fmt.Errorf("%s: status field expected to have type string, found %T, at json response %s", title, statusI, jsonOutput)
	}
	if status != "0x1" {
		return fmt.Errorf("%s: incorrect status code %s, at json response %s", title, status, jsonOutput)
	}
	return nil
}

func GetClient(rpcURL string) (ethclient.Client, error) {
	return ethclient.Dial(rpcURL)
}

func GetSigner(client ethclient.Client, prefundedPrivateKeyStr string) (*bind.TransactOpts, error) {
	prefundedPrivateKeyStr = strings.TrimPrefix(prefundedPrivateKeyStr, "0x")
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

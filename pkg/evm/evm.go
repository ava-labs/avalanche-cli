// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

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
	return nil
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
	return nil
}

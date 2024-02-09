// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

const (
	// TODO: use latest version
	teleporterVersion                          = "v0.1.0"
	teleporterReleaseURL                       = "https://github.com/ava-labs/teleporter/releases/download/" + teleporterVersion + "/"
	teleporterMessengerContractAddressURL      = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerAddressURL      = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerTxURL           = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_" + teleporterVersion + ".txt"
	cChainRpcURL                               = constants.LocalAPIEndpoint + "/ext/bc/C/rpc"
	prefundedEwoqPrivateKey                    = "0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
	teleporterMessengerDeployerRequiredBalance = uint64(10000000000000000000) // 10 eth
)

func DeployTeleporter(subnetName string, rpcURL string, prefundedPrivateKey string) error {
	ux.Logger.PrintToUser("Deploying Teleporter into %s", subnetName)
	// get target teleporter messenger contract address
	teleporterMessengerContractAddress, err := downloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return err
	}
	// check if contract is already deployed
	teleporterMessengerAlreadyDeployed, err := contractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
	if err != nil {
		return err
	}
	if teleporterMessengerAlreadyDeployed {
		return nil
	}
	// get teleporter deployer address
	teleporterMessengerDeployerAddress, err := downloadStr(teleporterMessengerDeployerAddressURL)
	if err != nil {
		return err
	}
	// get teleporter deployer balance
	teleporterMessengerDeployerBalance, err := getAddressBalance(rpcURL, teleporterMessengerDeployerAddress)
	if err != nil {
		return err
	}
	if teleporterMessengerDeployerBalance < teleporterMessengerDeployerRequiredBalance {
		toFund := teleporterMessengerDeployerRequiredBalance - teleporterMessengerDeployerBalance
		err := fundAddress(
			rpcURL,
			prefundedPrivateKey,
			teleporterMessengerDeployerAddress,
			toFund,
		)
		if err != nil {
			return err
		}
	}
	teleporterMessengerDeployerTx, err := downloadStr(teleporterMessengerDeployerTxURL)
	if err != nil {
		return err
	}
	err = issueTx(rpcURL, teleporterMessengerDeployerTx)
	if err != nil {
		return err
	}
	return nil
}

func TeleporterAlreadyDeployed(rpcURL string) (bool, error) {
	teleporterMessengerContractAddress, err := downloadStr(teleporterMessengerContractAddressURL)
	if err != nil {
		return false, err
	}
	return contractAlreadyDeployed(rpcURL, teleporterMessengerContractAddress)
}

func contractAlreadyDeployed(rpcURL string, contractAddress string) (bool, error) {
	bytecode, err := getContractBytecode(rpcURL, contractAddress)
	if err != nil {
		return false, err
	}
	return bytecode != "0x", nil
}

func getContractBytecode(rpcURL string, contractAddress string) (string, error) {
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

func getAddressBalance(rpcURL string, address string) (uint64, error) {
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

func fundAddress(rpcURL string, sourceAddressPrivateKey string, targetAddress string, amount uint64) error {
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

func issueTx(rpcURL string, tx string) error {
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

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed downloading %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed downloading %s: unexpected http status code: %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed downloading $s: %w", url, err)
	}
	return bs, nil
}

func downloadStr(url string) (string, error) {
	bs, err := download(url)
	return string(bs), err
}

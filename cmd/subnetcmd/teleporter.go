// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/spf13/cobra"
)

const (
	// TODO: use latest version
	teleporterVersion = "v0.1.0"
	teleporterReleaseURL = "https://github.com/ava-labs/teleporter/releases/download/" + teleporterVersion + "/"
	teleporterMessengerContractAddressURL = teleporterReleaseURL + "/TeleporterMessenger_Contract_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerAddressURL = teleporterReleaseURL + "/TeleporterMessenger_Deployer_Address_" + teleporterVersion + ".txt"
	teleporterMessengerDeployerTxURL = teleporterReleaseURL + "/TeleporterMessenger_Deployment_Transaction_" + teleporterVersion + ".txt"
	cChainRpcURL = constants.LocalAPIEndpoint + "/ext/bc/C/rpc"
	prefundedEwoqPrivateKey = "0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teleporter",
		Short: "Deploys teleporter to local network cchain",
		Long: `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	teleporterMessengerContractAddressBytes, err := download(teleporterMessengerContractAddressURL)
	if err != nil {
		return err
	}
	teleporterMessengerContractAddress := string(teleporterMessengerContractAddressBytes)
	teleporterMessengerAlreadyDeployed, err := contractAlreadyDeployed(cChainRpcURL, teleporterMessengerContractAddress)
	if err != nil {
		return err
	}
	if teleporterMessengerAlreadyDeployed {
		fmt.Printf("TELEPORTER MESSENGER ALREADY DEPLOYED TO RPC %s CONTRACT ADDRESS %s", cChainRpcURL, teleporterMessengerContractAddress)
		return nil
	}
	fmt.Printf("DEPLOYING TELEPORTER MESSENGER INTO RPC %s CONTRACT ADDRESS %s\n", cChainRpcURL, teleporterMessengerContractAddress)
	return nil

	teleporterMessengerDeployerAddress, err := download(teleporterMessengerDeployerAddressURL)
	if err != nil {
		return err
	}
	fmt.Println("(", string(teleporterMessengerDeployerAddress), ")")
	return nil
	teleporterMessengerDeployerTx, err := download(teleporterMessengerDeployerTxURL)
	if err != nil {
		return err
	}
	_ = teleporterMessengerDeployerTx
	return nil
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
	outBuf, errBuf:= utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		if outBuf.String() != "" {
			fmt.Println(outBuf.String())
		}
		if errBuf.String() != "" {
			fmt.Println(errBuf.String())
		}
		return "", fmt.Errorf("could get contract %s bytecode from rpc %s: %w", contractAddress, rpcURL, err)
	}
	if errBuf.String() != "" {
		fmt.Println(errBuf.String())
	}
	return strings.TrimSpace(outBuf.String()), nil
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

package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/coreth/ethclient"
)

func GetBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, baseDir)
}

func SubnetConfigExists(subnetName string) (bool, error) {
	genesis := path.Join(GetBaseDir(), subnetName+constants.GenesisSuffix)
	genesisExists := true
	if _, err := os.Stat(genesis); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		genesisExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	sidecar := path.Join(GetBaseDir(), subnetName+constants.SidecarSuffix)
	sidecarExists := true
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		sidecarExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	// do an xor
	if (genesisExists || sidecarExists) && !(genesisExists && sidecarExists) {
		return false, errors.New("config half exists")
	}
	return genesisExists && sidecarExists, nil
}

func DeleteConfigs(subnetName string) error {
	genesis := path.Join(GetBaseDir(), subnetName+constants.GenesisSuffix)
	if _, err := os.Stat(genesis); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	os.Remove(genesis)

	sidecar := path.Join(GetBaseDir(), subnetName+constants.SidecarSuffix)
	if _, err := os.Stat(sidecar); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	os.Remove(sidecar)

	return nil
}

func ClearBins() {
	binDirPath := path.Join(GetBaseDir(), binDir)
	fmt.Println("About to remove", binDirPath)
	// ignore error, files may not exist
	// os.RemoveAll(binDirPath)
}

func stdoutParser(output string, queue string, capture string) (string, error) {
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, queue) {
			index := strings.Index(line, capture)
			if index == -1 {
				return "", errors.New("capture string not available at queue")
			}
			return line[index:], nil
		}
	}
	return "", errors.New("no queue string found")
}

func ParseRPCFromDeployOutput(output string) (string, error) {
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "RPC URL:") {
			index := strings.Index(line, "http")
			if index == -1 {
				return "", errors.New("no url in RPC URL line")
			}
			return line[index:], nil
		}
	}
	return "", errors.New("no rpc url found")
}

func ParseRPCFromRestartOutput(output string) (string, error) {
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Endpoint at node") {
			index := strings.Index(line, "http")
			if index == -1 {
				return "", errors.New("no url in RPC URL line")
			}
			return line[index:], nil
		}
	}
	return "", errors.New("no rpc url found")
}

type greeterAddr struct {
	Greeter string
}

func ParseGreeterAddress(output string) error {
	addr, err := stdoutParser(output, "Greeter deployed to:", "0x")
	if err != nil {
		return err
	}
	greeter := greeterAddr{addr}
	file, err := json.MarshalIndent(greeter, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(greeterFile, file, 0o600)
}

type rpcFile struct {
	RPC string `json:"rpc"`
}

func SetHardhatRPC(rpc string) error {
	rpcFileData := rpcFile{
		RPC: rpc,
	}

	file, err := json.MarshalIndent(rpcFileData, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(rpcFilePath, file, 0o600)
}

func RunHardhatTests(test string) error {
	cmd := exec.Command("npx", "hardhat", "test", test, "--network", "subnet")
	cmd.Dir = hardhatDir
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	return err
}

func RunHardhatScript(script string) (string, string, error) {
	cmd := exec.Command("npx", "hardhat", "run", script, "--network", "subnet")
	cmd.Dir = hardhatDir
	output, err := cmd.Output()
	exitErr, typeOk := err.(*exec.ExitError)
	stderr := ""
	if typeOk {
		stderr = string(exitErr.Stderr)
	}
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	return string(output), stderr, err
}

func GetBlockHeight(rpc string) error {
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return err
	}

	number, err := client.BlockNumber(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("Block height", number)
	return nil
}

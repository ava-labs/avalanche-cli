// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	ledger "github.com/ava-labs/avalanche-ledger-go"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/spacesvm/chain"
	spacesvmclient "github.com/ava-labs/spacesvm/client"
	"github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	expectedRPCComponentsLen = 7
	blockchainIDPos          = 5
	subnetEVMName            = "subnet-evm"
)

func GetBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, baseDir)
}

func GetAPMDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, constants.APMDir)
}

func genesisExists(subnetName string) (bool, error) {
	genesis := path.Join(GetBaseDir(), subnetName+constants.GenesisSuffix)
	genesisExists := true
	if _, err := os.Stat(genesis); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		genesisExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return genesisExists, nil
}

func sidecarExists(subnetName string) (bool, error) {
	sidecar := path.Join(GetBaseDir(), subnetName+constants.SidecarSuffix)
	sidecarExists := true
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		sidecarExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return sidecarExists, nil
}

func SubnetConfigExists(subnetName string) (bool, error) {
	gen, err := genesisExists(subnetName)
	if err != nil {
		return false, err
	}

	sc, err := sidecarExists(subnetName)
	if err != nil {
		return false, err
	}

	// do an xor
	if (gen || sc) && !(gen && sc) {
		return false, errors.New("config half exists")
	}
	return gen && sc, nil
}

func APMConfigExists(subnetName string) (bool, error) {
	return sidecarExists(subnetName)
}

func SubnetCustomVMExists(subnetName string) (bool, error) {
	vm := path.Join(GetBaseDir(), constants.CustomVMDir, subnetName)
	vmExists := true
	if _, err := os.Stat(vm); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		vmExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return vmExists, nil
}

func SubnetAPMVMExists(subnetName string) (bool, error) {
	sidecarPath := path.Join(GetBaseDir(), subnetName+constants.SidecarSuffix)
	jsonBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		return false, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)
	if err != nil {
		return false, err
	}

	vmid := sc.ImportedVMID

	vm := path.Join(GetBaseDir(), constants.APMPluginDir, vmid)
	vmExists := true
	if _, err := os.Stat(vm); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		vmExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return vmExists, nil
}

func KeyExists(keyName string) (bool, error) {
	keyPath := path.Join(GetBaseDir(), constants.KeyDir, keyName+constants.KeySuffix)
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		return false, nil
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	return true, nil
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

func RemoveAPMRepo() {
	os.RemoveAll(GetAPMDir())
}

func DeleteKey(keyName string) error {
	keyPath := path.Join(GetBaseDir(), constants.KeyDir, keyName+constants.KeySuffix)
	if _, err := os.Stat(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	os.Remove(keyPath)

	return nil
}

func DeleteBins() error {
	avagoPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir)
	if _, err := os.Stat(avagoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	os.RemoveAll(avagoPath)

	subevmPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir)
	if _, err := os.Stat(subevmPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	os.RemoveAll(subevmPath)

	return nil
}

func DeleteAPMBin(vmid string) {
	vmPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.APMPluginDir, vmid)

	// ignore error, file may not exist
	os.RemoveAll(vmPath)
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

func ParseRPCsFromOutput(output string) ([]string, error) {
	rpcs := []string{}
	blockchainIDs := map[string]struct{}{}
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "rpc") {
			continue
		}
		startIndex := strings.Index(line, "http")
		if startIndex == -1 {
			return nil, errors.New("no url in RPC URL line")
		}
		endIndex := strings.LastIndex(line, "rpc")
		rpc := line[startIndex : endIndex+3]
		rpcComponents := strings.Split(rpc, "/")
		if len(rpcComponents) != expectedRPCComponentsLen {
			return nil, fmt.Errorf("unexpected number of components in url %q: expected %d got %d",
				rpc,
				expectedRPCComponentsLen,
				len(rpcComponents),
			)
		}
		blockchainID := rpcComponents[blockchainIDPos]
		_, ok := blockchainIDs[blockchainID]
		if !ok {
			blockchainIDs[blockchainID] = struct{}{}
			rpcs = append(rpcs, rpc)
		}
	}
	if len(rpcs) == 0 {
		return nil, errors.New("no RPCs where found")
	}
	return rpcs, nil
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

type confFile struct {
	RPC     string `json:"rpc"`
	ChainID string `json:"chainID"`
}

func SetHardhatRPC(rpc string) error {
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	chainIDBig, err := client.ChainID(ctx)
	cancel()
	if err != nil {
		return err
	}

	confFileData := confFile{
		RPC:     rpc,
		ChainID: chainIDBig.String(),
	}

	file, err := json.MarshalIndent(confFileData, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(confFilePath, file, 0o600)
}

func RunHardhatTests(test string) error {
	cmd := exec.Command("npx", "hardhat", "test", test, "--network", "subnet")
	cmd.Dir = hardhatDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	return err
}

func RunHardhatScript(script string) (string, string, error) {
	cmd := exec.Command("npx", "hardhat", "run", script, "--network", "subnet")
	cmd.Dir = hardhatDir
	output, err := cmd.CombinedOutput()
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

func PrintStdErr(err error) {
	exitErr, typeOk := err.(*exec.ExitError)
	if typeOk {
		fmt.Println(string(exitErr.Stderr))
	}
}

func CheckKeyEquality(keyPath1, keyPath2 string) (bool, error) {
	key1, err := os.ReadFile(keyPath1)
	if err != nil {
		return false, err
	}

	key2, err := os.ReadFile(keyPath2)
	if err != nil {
		return false, err
	}

	return string(key1) == string(key2), nil
}

func CheckSubnetEVMExists(version string) bool {
	subevmPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir, "subnet-evm-"+version)
	_, err := os.Stat(subevmPath)
	return err == nil
}

func CheckAvalancheGoExists(version string) bool {
	avagoPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir, "avalanchego-"+version)
	_, err := os.Stat(avagoPath)
	return err == nil
}

// Currently downloads subnet-evm, but that suffices to test the custom vm functionality
func DownloadCustomVMBin() (string, error) {
	targetDir := os.TempDir()
	subnetEVMVersion, err := binutils.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		subnetEVMName,
	))
	if err != nil {
		return "", err
	}
	subnetEVMDir, err := binutils.DownloadReleaseVersion(logging.NoLog{}, subnetEVMName, subnetEVMVersion, targetDir)
	if err != nil {
		return "", err
	}
	subnetEVMBin := path.Join(subnetEVMDir, subnetEVMName)
	if _, err := os.Stat(subnetEVMBin); errors.Is(err, os.ErrNotExist) {
		return "", errors.New("subnet evm bin file was not created")
	} else if err != nil {
		return "", err
	}
	return subnetEVMBin, nil
}

func ParsePublicDeployOutput(output string) (string, string, error) {
	lines := strings.Split(output, "\n")
	var (
		subnetID string
		rpcURL   string
	)
	for _, line := range lines {
		if !strings.Contains(line, "Subnet ID") && !strings.Contains(line, "RPC URL") {
			continue
		}
		words := strings.Split(line, "|")
		if len(words) != 4 {
			return "", "", errors.New("error parsing output: invalid number of words in line")
		}
		if strings.Contains(line, "Subnet ID") {
			subnetID = strings.TrimSpace(words[2])
		} else {
			rpcURL = strings.TrimSpace(words[2])
		}
	}
	if subnetID == "" || rpcURL == "" {
		return "", "", errors.New("information not found in output")
	}
	return subnetID, rpcURL, nil
}

func UpdateNodesWhitelistedSubnets(whitelistedSubnets string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, constants.RequestTimeout)
	resp, err := cli.Status(ctx)
	cancel()
	if err != nil {
		return err
	}
	for _, nodeName := range resp.ClusterInfo.NodeNames {
		ctx, cancel := context.WithTimeout(rootCtx, constants.RequestTimeout)
		_, err := cli.RestartNode(ctx, nodeName, client.WithWhitelistedSubnets(whitelistedSubnets))
		cancel()
		if err != nil {
			return err
		}
	}
	ctx, cancel = context.WithTimeout(rootCtx, constants.RequestTimeout)
	_, err = cli.Health(ctx)
	cancel()
	if err != nil {
		return err
	}
	return nil
}

type NodeInfo struct {
	ID         string
	PluginDir  string
	ConfigFile string
	URI        string
}

func GetNodesInfo() (map[string]NodeInfo, error) {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, constants.RequestTimeout)
	resp, err := cli.Status(ctx)
	cancel()
	if err != nil {
		return nil, err
	}
	nodesInfo := map[string]NodeInfo{}
	for nodeName, nodeInfo := range resp.ClusterInfo.NodeInfos {
		nodesInfo[nodeName] = NodeInfo{
			ID:         nodeInfo.Id,
			PluginDir:  nodeInfo.PluginDir,
			ConfigFile: path.Join(path.Dir(nodeInfo.LogDir), "config.json"),
			URI:        nodeInfo.Uri,
		}
	}
	return nodesInfo, nil
}

func GetWhilelistedSubnetsFromConfigFile(configFile string) (string, error) {
	fileBytes, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to load avalanchego config file %s: %w", configFile, err)
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(fileBytes, &avagoConfig); err != nil {
		return "", fmt.Errorf("failed to unpack the config file %s to JSON: %w", configFile, err)
	}
	whitelistedSubnetsIntf := avagoConfig["whitelisted-subnets"]
	whitelistedSubnets, ok := whitelistedSubnetsIntf.(string)
	if !ok {
		return "", fmt.Errorf("expected a string value, but got %T", whitelistedSubnetsIntf)
	}
	return whitelistedSubnets, nil
}

func WaitSubnetValidators(subnetIDStr string, nodeInfos map[string]NodeInfo) error {
	var uri string
	for _, nodeInfo := range nodeInfos {
		uri = nodeInfo.URI
		break
	}
	pClient := platformvm.NewClient(uri)
	subnetID, err := ids.FromString(subnetIDStr)
	if err != nil {
		return err
	}
	mainCtx, mainCtxCancel := context.WithTimeout(context.Background(), time.Second*30)
	defer mainCtxCancel()
	for {
		ready := true
		ctx, ctxCancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
		vs, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
		ctxCancel()
		if err != nil {
			return err
		}
		subnetValidators := map[string]struct{}{}
		for _, v := range vs {
			subnetValidators[v.NodeID.String()] = struct{}{}
		}
		for _, nodeInfo := range nodeInfos {
			if _, isValidator := subnetValidators[nodeInfo.ID]; !isValidator {
				ready = false
			}
		}
		if ready {
			return nil
		}
		select {
		case <-mainCtx.Done():
			return mainCtx.Err()
		case <-time.After(time.Second * 1):
		}
	}
}

func RunSpacesVMAPITest(rpc string) error {
	privHexBytes, err := os.ReadFile(EwoqKeyPath)
	if err != nil {
		return err
	}
	priv, err := crypto.HexToECDSA(strings.TrimSpace(string(privHexBytes)))
	if err != nil {
		return err
	}

	cli := spacesvmclient.New(strings.ReplaceAll(rpc, "/rpc", ""), constants.RequestTimeout)

	// claim a space
	space := "clispace"
	claimTx := &chain.ClaimTx{
		BaseTx: &chain.BaseTx{},
		Space:  space,
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	_, _, err = spacesvmclient.SignIssueRawTx(
		ctx,
		cli,
		claimTx,
		priv,
		spacesvmclient.WithPollTx(),
		spacesvmclient.WithInfo(space),
	)
	cancel()
	if err != nil {
		return err
	}

	// set key/val pair
	k, v := "key", []byte("value")
	setTx := &chain.SetTx{
		BaseTx: &chain.BaseTx{},
		Space:  space,
		Key:    k,
		Value:  v,
	}
	ctx, cancel = context.WithTimeout(context.Background(), constants.RequestTimeout)
	_, _, err = spacesvmclient.SignIssueRawTx(
		ctx,
		cli,
		setTx,
		priv,
		spacesvmclient.WithPollTx(),
		spacesvmclient.WithInfo(space),
	)
	cancel()
	if err != nil {
		return err
	}

	// check key/val pair
	_, rv, _, err := cli.Resolve(context.Background(), space+"/"+k)
	if err != nil {
		return err
	}
	if string(rv) != string(v) {
		return fmt.Errorf("expected value to be %q, got %q", v, rv)
	}
	return nil
}

func FundLedgerAddress() error {
	return nil
	// get ledger addr
	ledgerDev, err := ledger.Connect()
	if err != nil {
		return err
	}
	fmt.Println("*** Please provide extended public key on the ledger device ***")
	ledgerAddrs, err := ledgerDev.Addresses(1)
	if err != nil {
		return err
	}
	if len(ledgerAddrs) != 1 {
		return fmt.Errorf("no ledger addresses available")
	}
	ledgerAddr := ledgerAddrs[0]
	if err := ledgerDev.Disconnect(); err != nil {
		return err
	}

	// get genesis funded wallet
	sk, err := key.LoadSoft(constants.LocalNetworkID, EwoqKeyPath)
	if err != nil {
		return err
	}
	kc := sk.KeyChain()
	wallet, err := primary.NewWalletWithTxs(context.Background(), constants.LocalAPIEndpoint, kc)
	if err != nil {
		return err
	}

	// fund ledger on P-Chain by X export/P import
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{ledgerAddr},
	}
	output := &avax.TransferableOutput{
		Asset: avax.Asset{ID: wallet.X().AVAXAssetID()},
		Out: &secp256k1fx.TransferOutput{
			Amt:          1000000000,
			OutputOwners: to,
		},
	}
	outputs := []*avax.TransferableOutput{output}
	txID, err := wallet.X().IssueExportTx(avago_constants.PlatformChainID, outputs)
	if err != nil {
		return err
	}
	txID, err = wallet.P().IssueImportTx(wallet.X().BlockchainID(), &to)
	if err != nil {
		return err
	}
	_ = txID
	return nil
}

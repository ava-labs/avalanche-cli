// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/subnet"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/subnet-evm/ethclient"
)

const (
	expectedKeyListLineComponents = 8
	expectedRPCComponentsLen      = 7
	blockchainIDPos               = 5
	subnetEVMName                 = "subnet-evm"
)

var defaultLocalNetworkNodeIDs = []string{
	"NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg", "NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ",
	"NodeID-NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN", "NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu", "NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5",
}

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

func ChainConfigExists(subnetName string) (bool, error) {
	cfgPath := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.ChainConfigFileName)
	cfgExists := true
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		cfgExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return cfgExists, nil
}

func PerNodeChainConfigExists(subnetName string) (bool, error) {
	cfgPath := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.PerNodeChainConfigFileName)
	cfgExists := true
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		cfgExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return cfgExists, nil
}

func genesisExists(subnetName string) (bool, error) {
	genesis := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.GenesisFileName)
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
	sidecar := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.SidecarFileName)
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

func ElasticSubnetConfigExists(subnetName string) (bool, error) {
	elasticSubnetConfig := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.ElasticSubnetConfigFileName)
	elasticSubnetConfigExists := true
	if _, err := os.Stat(elasticSubnetConfig); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		elasticSubnetConfigExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
	return elasticSubnetConfigExists, nil
}

func PermissionlessValidatorExistsInSidecar(subnetName string, nodeID string, network string) (bool, error) {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return false, err
	}
	elasticSubnetValidators := sc.ElasticSubnet[network].Validators
	_, ok := elasticSubnetValidators[nodeID]
	return ok, nil
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

func AddSubnetIDToSidecar(subnetName string, network models.Network, subnetID string) error {
	exists, err := sidecarExists(subnetName)
	if err != nil {
		return fmt.Errorf("failed to access sidecar for %s: %w", subnetName, err)
	}
	if !exists {
		return fmt.Errorf("failed to access sidecar for %s: not found", subnetName)
	}

	sidecar := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.SidecarFileName)

	jsonBytes, err := os.ReadFile(sidecar)
	if err != nil {
		return err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)
	if err != nil {
		return err
	}

	subnetIDstr, err := ids.FromString(subnetID)
	if err != nil {
		return err
	}
	sc.Networks[network.String()] = models.NetworkData{
		SubnetID: subnetIDstr,
	}

	fileBytes, err := json.Marshal(&sc)
	if err != nil {
		return err
	}

	return os.WriteFile(sidecar, fileBytes, constants.DefaultPerms755)
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
	sidecarPath := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.SidecarFileName)
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
	subnetDir := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName)
	if _, err := os.Stat(subnetDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	_ = os.RemoveAll(subnetDir)

	return nil
}

func RemoveAPMRepo() {
	_ = os.RemoveAll(GetAPMDir())
}

func DeleteKey(keyName string) error {
	keyPath := path.Join(GetBaseDir(), constants.KeyDir, keyName+constants.KeySuffix)
	if _, err := os.Stat(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	_ = os.Remove(keyPath)

	return nil
}

func DeleteBins() error {
	avagoPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir)
	if _, err := os.Stat(avagoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	_ = os.RemoveAll(avagoPath)

	subevmPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir)
	if _, err := os.Stat(subevmPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Schrodinger: file may or may not exist. See err for details.
		return err
	}

	// ignore error, file may not exist
	_ = os.RemoveAll(subevmPath)

	return nil
}

func DeleteCustomBinary(vmName string) {
	vmPath := path.Join(GetBaseDir(), constants.VMDir, vmName)
	// ignore error, file may not exist
	_ = os.RemoveAll(vmPath)
}

func DeleteAPMBin(vmid string) {
	vmPath := path.Join(GetBaseDir(), constants.AvalancheCliBinDir, constants.APMPluginDir, vmid)

	// ignore error, file may not exist
	_ = os.RemoveAll(vmPath)
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
			return nil, fmt.Errorf("no url in RPC URL line: %s", line)
		}
		endIndex := strings.Index(line, "rpc")
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

func ParseAddrBalanceFromKeyListOutput(output string, keyName string) (string, uint64, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, keyName) {
			continue
		}
		components := strings.Split(line, "|")
		if len(components) != expectedKeyListLineComponents {
			return "", 0, fmt.Errorf("unexpected number of components in key list line %q: expected %d got %d",
				line,
				expectedKeyListLineComponents,
				len(components),
			)
		}
		addr := strings.TrimSpace(components[4])
		balanceStr := strings.TrimSpace(components[5])
		balance, err := strconv.ParseUint(balanceStr, 0, 64)
		if err != nil {
			return "", 0, fmt.Errorf("error parsing expected float %s", balanceStr)
		}
		return addr, balance, nil
	}
	return "", 0, fmt.Errorf("keyName %s not found in key list", keyName)
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
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
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
	var (
		exitErr *exec.ExitError
		stderr  string
	)
	if errors.As(err, &exitErr) {
		stderr = string(exitErr.Stderr)
	}
	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	}
	return string(output), stderr, err
}

func PrintStdErr(err error) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
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
func DownloadCustomVMBin(subnetEVMversion string) (string, error) {
	targetDir := os.TempDir()
	subnetEVMDir, err := binutils.DownloadReleaseVersion(logging.NoLog{}, subnetEVMName, subnetEVMversion, targetDir)
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

func ParsePublicDeployOutput(output string) (string, error) {
	lines := strings.Split(output, "\n")
	var subnetID string
	for _, line := range lines {
		if !strings.Contains(line, "Subnet ID") && !strings.Contains(line, "RPC URL") {
			continue
		}
		words := strings.Split(line, "|")
		if len(words) != 4 {
			return "", errors.New("error parsing output: invalid number of words in line")
		}
		if strings.Contains(line, "Subnet ID") {
			subnetID = strings.TrimSpace(words[2])
		}
	}
	if subnetID == "" {
		return "", errors.New("information not found in output")
	}
	return subnetID, nil
}

func RestartNodesWithWhitelistedSubnets(whitelistedSubnets string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, constants.E2ERequestTimeout)
	resp, err := cli.Status(ctx)
	cancel()
	if err != nil {
		return err
	}
	for _, nodeName := range resp.ClusterInfo.NodeNames {
		ctx, cancel := context.WithTimeout(rootCtx, constants.E2ERequestTimeout)
		_, err := cli.RestartNode(ctx, nodeName, client.WithWhitelistedSubnets(whitelistedSubnets))
		cancel()
		if err != nil {
			return err
		}
	}
	ctx, cancel = context.WithTimeout(rootCtx, constants.E2ERequestTimeout)
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
	LogDir     string
}

func GetNodeVMVersion(nodeURI string, vmid string) (string, error) {
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, constants.E2ERequestTimeout)

	client := info.NewClient(nodeURI)
	versionInfo, err := client.GetNodeVersion(ctx)
	cancel()
	if err != nil {
		return "", err
	}

	for vm, version := range versionInfo.VMVersions {
		if vm == vmid {
			return version, nil
		}
	}
	return "", errors.New("vmid not found")
}

func GetNodesInfo() (map[string]NodeInfo, error) {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, constants.E2ERequestTimeout)
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
			LogDir:     nodeInfo.LogDir,
		}
	}
	return nodesInfo, nil
}

func GetWhitelistedSubnetsFromConfigFile(configFile string) (string, error) {
	fileBytes, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to load avalanchego config file %s: %w", configFile, err)
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(fileBytes, &avagoConfig); err != nil {
		return "", fmt.Errorf("failed to unpack the config file %s to JSON: %w", configFile, err)
	}
	whitelistedSubnetsIntf := avagoConfig["track-subnets"]
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
		ctx, ctxCancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
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

func GetFileHash(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func FundLedgerAddress() error {
	// get ledger
	ledgerDev, err := ledger.New()
	if err != nil {
		return err
	}

	// get ledger addr
	ledgerAddrs, err := ledgerDev.Addresses([]uint32{0})
	if err != nil {
		return err
	}
	if len(ledgerAddrs) != 1 {
		return fmt.Errorf("no ledger addresses available")
	}
	ledgerAddr := ledgerAddrs[0]

	// get genesis funded wallet
	sk, err := key.LoadSoft(constants.LocalNetworkID, EwoqKeyPath)
	if err != nil {
		return err
	}
	var kc keychain.Keychain
	kc = sk.KeyChain()
	wallet, err := primary.NewWalletWithTxs(context.Background(), constants.LocalAPIEndpoint, kc)
	if err != nil {
		return err
	}

	// export X-Chain genesis addr to P-Chain ledger addr
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
	if _, err := wallet.X().IssueExportTx(avago_constants.PlatformChainID, outputs); err != nil {
		return err
	}

	// get ledger funded wallet
	kc, err = keychain.NewLedgerKeychain(ledgerDev, 1)
	if err != nil {
		return err
	}
	wallet, err = primary.NewWalletWithTxs(context.Background(), constants.LocalAPIEndpoint, kc)
	if err != nil {
		return err
	}

	// import X-Chain genesis addr to P-Chain ledger addr
	fmt.Println("*** Please sign import hash on the ledger device *** ")
	if _, err = wallet.P().IssueImportTx(wallet.X().BlockchainID(), &to); err != nil {
		return err
	}

	if err := ledgerDev.Disconnect(); err != nil {
		return err
	}

	return nil
}

func GetPluginBinaries() ([]string, error) {
	// load plugin files from the plugin directory
	pluginDir := path.Join(GetBaseDir(), PluginDirExt)
	files, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, err
	}

	pluginFiles := []string{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		pluginFiles = append(pluginFiles, filepath.Join(pluginDir, file.Name()))
	}

	return pluginFiles, nil
}

func getSideCar(subnetName string) (models.Sidecar, error) {
	exists, err := sidecarExists(subnetName)
	if err != nil {
		return models.Sidecar{}, fmt.Errorf("failed to access sidecar for %s: %w", subnetName, err)
	}
	if !exists {
		return models.Sidecar{}, fmt.Errorf("failed to access sidecar for %s: not found", subnetName)
	}

	sidecar := filepath.Join(GetBaseDir(), constants.SubnetDir, subnetName, constants.SidecarFileName)

	jsonBytes, err := os.ReadFile(sidecar)
	if err != nil {
		return models.Sidecar{}, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)
	if err != nil {
		return models.Sidecar{}, err
	}
	return sc, nil
}

func GetValidators(subnetName string) ([]string, error) {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return nil, err
	}
	subnetID := sc.Networks[models.Local.String()].SubnetID
	if subnetID == ids.Empty {
		return nil, errors.New("no subnet id")
	}
	// Get NodeIDs of all validators on the subnet
	validators, err := subnet.GetSubnetValidators(subnetID)
	if err != nil {
		return nil, err
	}
	nodeIDsList := []string{}
	for _, validator := range validators {
		nodeIDsList = append(nodeIDsList, validator.NodeID.String())
	}
	return nodeIDsList, nil
}

func GetCurrentSupply(subnetName string) error {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[models.Local.String()].SubnetID
	return subnet.GetCurrentSupply(subnetID)
}

func IsNodeInPendingValidator(subnetName string, nodeID string) (bool, error) {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return false, err
	}
	subnetID := sc.Networks[models.Local.String()].SubnetID
	return subnet.CheckNodeIsInSubnetPendingValidators(subnetID, nodeID)
}

func CheckAllNodesAreCurrentValidators(subnetName string) (bool, error) {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return false, err
	}
	subnetID := sc.Networks[models.Local.String()].SubnetID

	api := constants.LocalAPIEndpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()

	validators, err := pClient.GetCurrentValidators(ctx, subnetID, nil)
	if err != nil {
		return false, err
	}

	for _, nodeIDstr := range defaultLocalNetworkNodeIDs {
		currentValidator := false
		for _, validator := range validators {
			if validator.NodeID.String() == nodeIDstr {
				currentValidator = true
			}
		}
		if !currentValidator {
			return false, fmt.Errorf("%s is still not a current validator of the elastic subnet", nodeIDstr)
		}
	}
	return true, nil
}

func AllPermissionlessValidatorExistsInSidecar(subnetName string, network string) (bool, error) {
	sc, err := getSideCar(subnetName)
	if err != nil {
		return false, err
	}
	elasticSubnetValidators := sc.ElasticSubnet[network].Validators
	for _, nodeIDstr := range defaultLocalNetworkNodeIDs {
		_, ok := elasticSubnetValidators[nodeIDstr]
		if !ok {
			return false, err
		}
	}
	return true, nil
}

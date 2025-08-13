// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"bufio"
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

	avalanchegojson "github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/rpc"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/subnet-evm/ethclient"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	expectedKeyListLineComponents = 9
	expectedRPCComponentsLen      = 7
	blockchainIDPos               = 5
	subnetEVMName                 = "subnet-evm"
)

func GetBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, baseDir)
}

func GetSubnetDir() string {
	return path.Join(GetBaseDir(), constants.SubnetDir)
}

func GetAPMDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, constants.APMDir)
}

func GetSnapshotsDir() string {
	return filepath.Join(GetBaseDir(), constants.SnapshotsDirName)
}

func GetSnapshotPath(snapshotName string) string {
	return filepath.Join(GetSnapshotsDir(), snapshotName)
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
	sc.Networks[network.Name()] = models.NetworkData{
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

func DeleteSnapshot(snapshotName string) {
	snapshotPath := path.Join(GetSnapshotsDir(), snapshotName)

	// ignore error, file may not exist
	_ = os.RemoveAll(snapshotPath)
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

func ParseBlockchainIDFromOutput(output string) (string, error) {
	rpcs, err := ParseRPCsFromOutput(output)
	if err != nil {
		return "", err
	}
	if len(rpcs) == 0 {
		return "", fmt.Errorf("deploy output has no rpc info")
	}
	rpc := rpcs[0]
	rpcParts := strings.Split(rpc, "/")
	if len(rpcParts) != 7 {
		return "", fmt.Errorf("rpc at deploy output has inconsistent format: %s", rpc)
	}
	return rpcParts[5], nil
}

func ParseRPCsFromOutput(output string) ([]string, error) {
	rpcs := []string{}
	blockchainIDs := map[string]struct{}{}
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Waiting") {
			continue
		}
		if !strings.Contains(line, "rpc") {
			continue
		}
		if !strings.Contains(line, "http") {
			continue
		}
		startIndex := strings.Index(line, "http")
		if startIndex == -1 {
			return nil, fmt.Errorf("no url in RPC URL line: %s", line)
		}
		endIndex := strings.Index(line, "rpc")
		if startIndex > endIndex+3 {
			return nil, fmt.Errorf("unexpected format while looking for RPC info on output: %s", line)
		}
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
		return nil, errors.New("no RPCs were found")
	}
	return rpcs, nil
}

func ParseAddrBalanceFromKeyListOutput(output string, keyName string, subnet string) (string, uint64, error) {
	lines := strings.Split(output, "\n")
	keyFound := false
	for _, line := range lines {
		if strings.Contains(line, keyName) {
			keyFound = true
		}

		if keyFound && strings.Contains(line, subnet) {
			components := strings.Split(line, "|")
			if len(components) != expectedKeyListLineComponents {
				return "", 0, fmt.Errorf("unexpected number of components in key list line %q: expected %d got %d",
					line,
					expectedKeyListLineComponents,
					len(components),
				)
			}
			addr := strings.TrimSpace(components[4])
			balanceStr := strings.TrimSpace(components[6])

			var balance uint64
			if strings.Contains(balanceStr, ".") {
				balanceFloat, err := strconv.ParseFloat(balanceStr, 64)
				if err != nil {
					return "", 0, fmt.Errorf("error parsing expected float %s", balanceStr)
				}
				return addr, uint64(balanceFloat), nil
			}

			balance, err := strconv.ParseUint(balanceStr, 0, 64)
			if err != nil {
				return "", 0, fmt.Errorf("error parsing expected float %s", balanceStr)
			}

			return addr, balance, nil
		}
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

	return os.WriteFile(greeterFile, file, constants.WriteReadUserOnlyPerms)
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
	ctx, cancel := sdkutils.GetAPIContext()
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

	return os.WriteFile(confFilePath, file, constants.WriteReadUserOnlyPerms)
}

func StartLedgerSim(
	iters int,
	seed string,
	showStdout bool,
) (chan struct{}, chan struct{}) {
	ledgerSimReadyCh := make(chan struct{})
	interactionEndCh := make(chan struct{})
	ledgerSimEndCh := make(chan struct{})
	go func() {
		defer ginkgo.GinkgoRecover()
		err := RunLedgerSim(iters, seed, ledgerSimReadyCh, interactionEndCh, ledgerSimEndCh, showStdout)
		if err != nil {
			fmt.Println(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
	}()
	<-ledgerSimReadyCh
	return interactionEndCh, ledgerSimEndCh
}

func RunLedgerSim(
	iters int,
	seed string,
	ledgerSimReadyCh chan struct{},
	interactionEndCh chan struct{},
	ledgerSimEndCh chan struct{},
	showStdout bool,
) error {
	cmd := exec.Command(
		"npx",
		"tsx",
		basicLedgerSimScript,
		fmt.Sprintf("%d", iters),
		seed,
	)
	cmd.Dir = ledgerSimDir

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}

	go func(p io.ReadCloser) {
		reader := bufio.NewReader(p)
		line, err := reader.ReadString('\n')
		for err == nil {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "SIMULATED LEDGER DEV READY") {
				close(ledgerSimReadyCh)
			}
			if strings.Contains(line, "PRESS ENTER TO END SIMULATOR") {
				<-interactionEndCh
				_, _ = io.WriteString(stdinPipe, "\n")
			}
			if showStdout {
				fmt.Println(line)
			}
			line, err = reader.ReadString('\n')
		}
	}(stdoutPipe)

	stderr, err := io.ReadAll(stderrPipe)
	if err != nil {
		return err
	}
	if len(stderr) != 0 {
		fmt.Println(string(stderr))
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}

	// close if previous one failed
	select {
	case <-ledgerSimReadyCh:
	default:
		close(ledgerSimReadyCh)
	}

	close(ledgerSimEndCh)

	return err
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
		if string(exitErr.Stderr) != "" {
			fmt.Println(string(exitErr.Stderr))
		}
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

func CheckSnapshotExists(snapshotName string) bool {
	snapshotPath := filepath.Join(GetSnapshotsDir(), snapshotName)
	_, err := os.Stat(snapshotPath)
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

// ParsePublicDeployOutput can parse Subnet ID or Blockchain ID
func ParsePublicDeployOutput(output string, parseType string) (string, error) {
	lines := strings.Split(output, "\n")
	var targetID string
	for _, line := range lines {
		if !strings.Contains(line, "Subnet ID") && !strings.Contains(line, "RPC URL") && !strings.Contains(line, " Blockchain ID") {
			continue
		}
		words := strings.Split(line, "|")
		if len(words) != 4 {
			return "", errors.New("error parsing output: invalid number of words in line")
		}
		if parseType == SubnetIDParseType {
			if strings.Contains(line, "Subnet ID") {
				targetID = strings.TrimSpace(words[2])
			}
		}
		if parseType == BlockchainIDParseType {
			if strings.Contains(line, "Blockchain ID") {
				targetID = strings.TrimSpace(words[2])
			}
		}
	}
	if targetID == "" {
		return "", errors.New("information not found in output")
	}
	return targetID, nil
}

func RestartNodes() error {
	network, err := GetLocalNetwork()
	if err != nil {
		return err
	}
	ctx, cancel := localnet.GetLocalNetworkDefaultContext()
	defer cancel()
	if err := localnet.TmpNetRestartNodes(
		ctx,
		logging.NoLog{},
		func(string, ...interface{}) {},
		network,
		network.Nodes,
		nil,
	); err != nil {
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
	ctx, cancel := sdkutils.GetAPIContext()

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

func GetApp() *application.Avalanche {
	app := application.New()
	app.Setup(GetBaseDir(), logging.NoLog{}, nil, "", nil, nil, nil)
	return app
}

func GetLocalNetwork() (*tmpnet.Network, error) {
	app := GetApp()
	return localnet.GetLocalNetwork(app)
}

func GetLocalNetworkNodesInfo() (map[string]NodeInfo, error) {
	network, err := GetLocalNetwork()
	if err != nil {
		return nil, err
	}
	return getNodesInfo(network)
}

func GetLocalClusterName() (string, error) {
	app := GetApp()
	clusters, err := localnet.GetRunningLocalClustersConnectedToLocalNetwork(app)
	if err != nil {
		return "", err
	}
	if len(clusters) != 1 {
		return "", fmt.Errorf("expected 1 local network cluster running, found %d", len(clusters))
	}
	return clusters[0], nil
}

func GetLocalCluster() (*tmpnet.Network, error) {
	app := GetApp()
	clusterName, err := GetLocalClusterName()
	if err != nil {
		return nil, err
	}
	return localnet.GetLocalCluster(app, clusterName)
}

func GetLocalClusterNodesInfo() (map[string]NodeInfo, error) {
	network, err := GetLocalCluster()
	if err != nil {
		return nil, err
	}
	return getNodesInfo(network)
}

func getNodesInfo(network *tmpnet.Network) (map[string]NodeInfo, error) {
	pluginDir, err := network.DefaultFlags.GetStringVal(config.PluginDirKey)
	if err != nil {
		return nil, err
	}
	nodesInfo := map[string]NodeInfo{}
	for _, node := range network.Nodes {
		nodeID := node.NodeID.String()
		nodesInfo[nodeID] = NodeInfo{
			ID:         nodeID,
			PluginDir:  pluginDir,
			ConfigFile: path.Join(network.Dir, nodeID, "config.json"),
			URI:        node.URI,
			LogDir:     path.Join(network.Dir, nodeID, "logs"),
		}
	}
	return nodesInfo, nil
}

func GetLocalClusterUris() ([]string, error) {
	app := GetApp()
	clusterName, err := GetLocalClusterName()
	if err != nil {
		return nil, err
	}
	return localnet.GetLocalClusterURIs(app, clusterName)
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
	mainCtx, mainCtxCancel := sdkutils.GetAPIContext()
	defer mainCtxCancel()
	for {
		ready := true
		ctx, ctxCancel := sdkutils.GetAPIContext()
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

func GetLedgerAddress(network models.Network, index uint32) (string, error) {
	// get ledger
	ledgerDev, err := ledger.New()
	if err != nil {
		return "", err
	}
	// get ledger addr
	ledgerAddrs, err := ledgerDev.Addresses([]uint32{index})
	if err != nil {
		return "", err
	}
	if len(ledgerAddrs) != 1 {
		return "", fmt.Errorf("no ledger addresses available")
	}
	ledgerAddr := ledgerAddrs[0]
	hrp := key.GetHRP(network.ID)
	ledgerAddrStr, err := address.Format("P", hrp, ledgerAddr[:])
	if err != nil {
		return "", err
	}
	return ledgerAddrStr, nil
}

func FundLedgerAddress(amount uint64) error {
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
	if err := ledgerDev.Disconnect(); err != nil {
		return err
	}
	return FundAddress(ledgerAddr, amount)
}

func FundAddress(addr ids.ShortID, amount uint64) error {
	ctx, cancel := sdkutils.GetTimedContext(constants.WalletCreationTimeout)
	defer cancel()
	// get genesis funded wallet
	sk, err := key.LoadSoft(constants.LocalNetworkID, EwoqKeyPath)
	if err != nil {
		return err
	}
	kc := sk.KeyChain()
	wallet, err := primary.MakeWallet(
		ctx,
		constants.LocalAPIEndpoint,
		kc,
		secp256k1fx.NewKeychain(),
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	// transfer from P-Chain genesis addr to addr
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}
	output := &avax.TransferableOutput{
		Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt:          amount,
			OutputOwners: to,
		},
	}
	outputs := []*avax.TransferableOutput{output}
	if _, err := wallet.P().IssueBaseTx(outputs); err != nil {
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

func GetSideCar(subnetName string) (models.Sidecar, error) {
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

func GetSubnetEVMMainneChainID(subnetName string) (uint, error) {
	sc, err := GetSideCar(subnetName)
	if err != nil {
		return 0, err
	}
	return sc.SubnetEVMMainnetChainID, nil
}

func IsCustomVM(subnetName string) (bool, error) {
	sc, err := GetSideCar(subnetName)
	if err != nil {
		return false, err
	}
	return sc.VM == models.CustomVM, nil
}

// Get NodeIDs of all validators on the subnet
func GetSubnetValidators(subnetID ids.ID) ([]string, error) {
	network := models.NewLocalNetwork()
	validators, err := subnet.GetSubnetValidators(network, subnetID)
	if err != nil {
		return nil, err
	}
	nodeIDsList := []string{}
	for _, validator := range validators {
		nodeIDsList = append(nodeIDsList, validator.NodeID.String())
	}
	return nodeIDsList, nil
}

func GetTmpFilePath(fnamePrefix string) (string, error) {
	file, err := os.CreateTemp("", fnamePrefix+"*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	err = file.Close()
	if err != nil {
		return "", err
	}
	err = os.Remove(path)
	return path, err
}

func CreateTmpFile(fnamePrefix string, data []byte) (string, error) {
	file, err := os.CreateTemp("", fnamePrefix+"*")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(file.Name(), data, constants.DefaultPerms755); err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func ExecCommand(cmdName string, args []string, showStdout bool, errorIsExpected bool) string {
	cmd := exec.Command(cmdName, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	stderrPipe, err := cmd.StderrPipe()
	gomega.Expect(err).Should(gomega.BeNil())
	err = cmd.Start()
	gomega.Expect(err).Should(gomega.BeNil())

	stdout := ""
	go func(p io.ReadCloser) {
		reader := bufio.NewReader(p)
		line, err := reader.ReadString('\n')
		for err == nil {
			stdout += line
			if showStdout {
				fmt.Print(line)
			}
			line, err = reader.ReadString('\n')
		}
	}(stdoutPipe)

	stderr, err := io.ReadAll(stderrPipe)
	gomega.Expect(err).Should(gomega.BeNil())
	if len(stderr) != 0 {
		fmt.Println(string(stderr))
	}

	err = cmd.Wait()
	if errorIsExpected {
		gomega.Expect(err).Should(gomega.HaveOccurred())
	} else {
		gomega.Expect(err).Should(gomega.BeNil())
	}

	return stdout + string(stderr)
}

func GetKeyTransferFee(output string, network string) (uint64, error) {
	substr := fmt.Sprintf("%s Paid fee", network)
	feeNAvax := uint64(1)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			lineFields := strings.Fields(line)
			if len(lineFields) < 3 {
				return 0, fmt.Errorf("incorrect format for fee output of key transfer: %s", line)
			}
			feeAvaxStr := lineFields[3]
			feeAvax, err := strconv.ParseFloat(feeAvaxStr, 64)
			if err != nil {
				return 0, err
			}
			feeAvax *= float64(units.Avax)
			feeNAvax = uint64(feeAvax)
		}
	}
	return feeNAvax, nil
}

func GetE2EHostInstanceID() (string, error) {
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(path.Join(GetBaseDir(), constants.NodesDir, constants.AnsibleInventoryDir, constants.E2EClusterName))
	if err != nil {
		return "", err
	}
	_, cloudHostID, _ := models.HostAnsibleIDToCloudID(hosts[0].NodeID)
	return cloudHostID, nil
}

func GetERC20TokenAddress(output string) (string, error) {
	substr := "Token Address: "
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			lineFields := strings.Fields(line)
			if len(lineFields) < 3 {
				return "", fmt.Errorf("incorrect format for token address output: %s", line)
			}
			tokenAddress := lineFields[2]
			return tokenAddress, nil
		}
	}

	return "", fmt.Errorf("token address not found in output")
}

func GetTokenTransferrerAddresses(output string) (string, string, error) {
	substr := "Home Address: "
	var homeAddress string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			lineFields := strings.Fields(line)
			if len(lineFields) < 3 {
				return "", "", fmt.Errorf("incorrect format for token address output: %s", line)
			}
			homeAddress = lineFields[2]
		}
	}

	if homeAddress == "" {
		return "", "", fmt.Errorf("home address not found in output")
	}

	substr = "Remote Address: "
	var remoteAddress string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			lineFields := strings.Fields(line)
			if len(lineFields) < 3 {
				return "", "", fmt.Errorf("incorrect format for token address output: %s", line)
			}
			remoteAddress = lineFields[2]
		}
	}

	if remoteAddress == "" {
		return "", "", fmt.Errorf("remote address not found in output")
	}

	return homeAddress, remoteAddress, nil
}

type CurrentValidatorInfo struct {
	Weight       avalanchegojson.Uint64 `json:"weight"`
	NodeID       ids.NodeID             `json:"nodeID"`
	ValidationID ids.ID                 `json:"validationID"`
	Balance      avalanchegojson.Uint64 `json:"balance"`
}

func GetCurrentValidatorsLocalAPI(subnetID ids.ID) ([]CurrentValidatorInfo, error) {
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	requester := rpc.NewEndpointRequester("http://127.0.0.1:9650/ext/P")
	res := &platformvm.GetCurrentValidatorsReply{}
	if err := requester.SendRequest(
		ctx,
		"platform.getCurrentValidators",
		&platformvm.GetCurrentValidatorsArgs{
			SubnetID: subnetID,
			NodeIDs:  nil,
		},
		res,
	); err != nil {
		return nil, err
	}
	validators := make([]CurrentValidatorInfo, 0, len(res.Validators))
	for _, vI := range res.Validators {
		vBytes, err := json.Marshal(vI)
		if err != nil {
			return nil, err
		}
		var v CurrentValidatorInfo
		if err := json.Unmarshal(vBytes, &v); err != nil {
			return nil, err
		}
		validators = append(validators, v)
	}
	return validators, nil
}

func GetL1ValidatorInfo(validationID ids.ID) (platformvm.GetL1ValidatorReply, error) {
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	requester := rpc.NewEndpointRequester("http://127.0.0.1:9650/ext/P")
	res := &platformvm.GetL1ValidatorReply{}
	if err := requester.SendRequest(
		ctx,
		"platform.getL1Validator",
		&platformvm.GetL1ValidatorArgs{
			ValidationID: validationID,
		},
		res,
	); err != nil {
		return *res, err
	}
	return *res, nil
}

// clean up avalanchego logs for the given [nodesInfo]
// clean main.log and [blockchainID].log
func CleanupLogs(nodesInfo map[string]NodeInfo, blockchainID string) {
	for _, nodeInfo := range nodesInfo {
		logFile := path.Join(nodeInfo.LogDir, "main.log")
		err := os.Remove(logFile)
		gomega.Expect(err).Should(gomega.BeNil())
		if blockchainID != "" {
			logFile = path.Join(nodeInfo.LogDir, blockchainID+".log")
			err = os.Remove(logFile)
			gomega.Expect(err).Should(gomega.BeNil())
		}
	}
}

func ParseICMContractAddressesFromOutput(subnet, output string) (string, string, error) {
	var messengerAddress string
	var registryAddress string

	// split output by newline
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "ICM Messenger successfully deployed to "+subnet) {
			startIndex := strings.Index(line, "(")
			endIndex := strings.Index(line, ")")
			if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
				return "", "", fmt.Errorf("invalid format for contract address line: %s", line)
			}
			messengerAddress = strings.TrimSpace(line[startIndex+1 : endIndex])
		}

		if strings.Contains(line, "ICM Registry successfully deployed to "+subnet) {
			startIndex := strings.Index(line, "(")
			endIndex := strings.Index(line, ")")
			if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
				return "", "", fmt.Errorf("invalid format for contract address line: %s", line)
			}
			registryAddress = strings.TrimSpace(line[startIndex+1 : endIndex])
		}
	}

	if messengerAddress == "" && registryAddress == "" {
		return "", "", fmt.Errorf("messenger address not found in output")
	}

	return messengerAddress, registryAddress, nil
}

func ParseValidatorManagerAddressesFromOutput(output string) (string, string, string, error) {
	var validatorManagerAddress string
	var proxyAddress string
	var proxyAdminAddress string

	// split output by newline
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "Validator Manager Address: ") {
			startIndex := strings.Index(line, ":")
			if startIndex == -1 {
				return "", "", "", fmt.Errorf("invalid format for contract address line: %s", line)
			}
			endIndex := len(line)
			validatorManagerAddress = strings.TrimSpace(line[startIndex+1 : endIndex])
		}
		if strings.Contains(line, "Proxy Address: ") {
			startIndex := strings.Index(line, ":")
			if startIndex == -1 {
				return "", "", "", fmt.Errorf("invalid format for contract address line: %s", line)
			}
			endIndex := len(line)
			proxyAddress = strings.TrimSpace(line[startIndex+1 : endIndex])
		}
		if strings.Contains(line, "Proxy Admin Address: ") {
			startIndex := strings.Index(line, ":")
			if startIndex == -1 {
				return "", "", "", fmt.Errorf("invalid format for contract address line: %s", line)
			}
			endIndex := len(line)
			proxyAdminAddress = strings.TrimSpace(line[startIndex+1 : endIndex])
		}
	}

	if validatorManagerAddress == "" {
		return "", "", "", fmt.Errorf("messenger address not found in output")
	}

	return validatorManagerAddress, proxyAddress, proxyAdminAddress, nil
}

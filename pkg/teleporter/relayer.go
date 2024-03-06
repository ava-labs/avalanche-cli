// Copyright (C) 2022, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package teleporter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/awm-relayer/config"
	offchainregistry "github.com/ava-labs/awm-relayer/messages/off-chain-registry"
)

var teleporterRelayerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX

func GetRelayerKeyInfo(keyPath string) (string, string, error) {
	var (
		k   *key.SoftKey
		err error
	)
	if utils.FileExists(keyPath) {
		ux.Logger.PrintToUser("Loading stored key %q for relayer ops", constants.AWMRelayerKeyName)
		k, err = key.LoadSoft(models.LocalNetwork.ID, keyPath)
		if err != nil {
			return "", "", err
		}
	} else {
		ux.Logger.PrintToUser("Generating stored key %q for relayer ops", constants.AWMRelayerKeyName)
		k, err = key.NewSoft(0)
		if err != nil {
			return "", "", err
		}
		if err := k.Save(keyPath); err != nil {
			return "", "", err
		}
	}
	return k.C(), hex.EncodeToString(k.Raw()), nil
}

func FundRelayer(
	rpcURL string,
	prefundedPrivateKey string,
	teleporterRelayerAddress string,
) error {
	// get teleporter relayer balance
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	teleporterRelayerBalance, err := evm.GetAddressBalance(client, teleporterRelayerAddress)
	if err != nil {
		return err
	}
	if teleporterRelayerBalance.Cmp(teleporterRelayerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(teleporterRelayerRequiredBalance, teleporterRelayerBalance)
		err := evm.FundAddress(
			client,
			prefundedPrivateKey,
			teleporterRelayerAddress,
			toFund,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

type relayerRunFile struct {
	Pid int `json:"pid"`
}

func DeployRelayer(
	binDir string,
	configPath string,
	logFilePath string,
	runFilePath string,
	storageDir string,
) error {
	if err := RelayerCleanup(runFilePath, storageDir); err != nil {
		return err
	}
	downloader := application.NewDownloader()
	version, err := downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(constants.AvaLabsOrg, constants.AWMRelayerRepoName))
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("using latest awm-relayer version (%s)", version)
	versionBinDir := filepath.Join(binDir, version)
	binPath, err := installRelayer(versionBinDir, version)
	if err != nil {
		return err
	}
	pid, err := executeRelayer(binPath, configPath, logFilePath)
	if err != nil {
		return err
	}
	return saveRelayerRunFile(runFilePath, pid)
}

func RelayerCleanup(runFilePath string, storageDir string) error {
	if err := os.RemoveAll(storageDir); err != nil {
		return err
	}
	if !utils.FileExists(runFilePath) {
		return nil
	}
	bs, err := os.ReadFile(runFilePath)
	if err != nil {
		return err
	}
	rf := relayerRunFile{}
	if err := json.Unmarshal(bs, &rf); err != nil {
		return err
	}
	proc, err := os.FindProcess(rf.Pid)
	if err != nil {
		// after a reboot without network cleanup, it is expected that the file pid will exist but the process not
		return removeRelayerRunFile(runFilePath)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// after a reboot without network cleanup, it is expected that the file pid will exist but the process not
		// sometimes FindProcess returns without error, but Signal 0 will surely fail if the process doesn't exist
		return removeRelayerRunFile(runFilePath)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		ux.Logger.PrintToUser("failed trying to kill awm relayer with SIGINT. Using SIGKILL instead")
		if err := proc.Signal(os.Kill); err != nil {
			return fmt.Errorf("failed killing relayer process with pid %d: %w", rf.Pid, err)
		}
	}
	return removeRelayerRunFile(runFilePath)
}

func removeRelayerRunFile(runFilePath string) error {
	err := os.Remove(runFilePath)
	if err != nil {
		err = fmt.Errorf("failed removing relayer run file %s: %w", runFilePath, err)
	}
	return err
}

func saveRelayerRunFile(runFilePath string, pid int) error {
	rf := relayerRunFile{
		Pid: pid,
	}
	bs, err := json.Marshal(&rf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(runFilePath, bs, constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("could not write awm relater run file to %s: %w", runFilePath, err)
	}
	return nil
}

func installRelayer(binDir, version string) (string, error) {
	binPath := filepath.Join(binDir, constants.AWMRelayerBin)
	if utils.IsExecutable(binPath) {
		return binPath, nil
	}
	ux.Logger.PrintToUser("Installing AWM-Relayer %s", version)
	url, err := getRelayerURL(version)
	if err != nil {
		return "", err
	}
	bs, err := utils.Download(url)
	if err != nil {
		return "", err
	}
	if err := binutils.InstallArchive("tar.gz", bs, binDir); err != nil {
		return "", err
	}
	return binPath, nil
}

func executeRelayer(binPath string, configPath string, logFile string) (int, error) {
	logWriter, err := os.Create(logFile)
	if err != nil {
		return 0, err
	}

	ux.Logger.PrintToUser("Executing AWM-Relayer...")

	cmd := exec.Command(binPath, "--config-file", configPath)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	return cmd.Process.Pid, nil
}

func getRelayerURL(version string) (string, error) {
	goarch, goos := runtime.GOARCH, runtime.GOOS
	if goos != "linux" && goos != "darwin" {
		return "", fmt.Errorf("OS not supported: %s", goos)
	}
	trimmedVersion := strings.TrimPrefix(version, "v")
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/awm-relayer_%s_%s_%s.tar.gz",
		constants.AvaLabsOrg,
		constants.AWMRelayerRepoName,
		version,
		trimmedVersion,
		goos,
		goarch,
	), nil
}

func UpdateRelayerConfig(
	relayerConfigPath string,
	relayerStorageDir string,
	relayerAddress string,
	relayerPrivateKey string,
	network models.Network,
	subnetID string,
	blockchainID string,
	teleporterContractAddress string,
	teleporterRegistryAddress string,
) error {
	awmRelayerConfig := config.Config{}
	if utils.FileExists(relayerConfigPath) {
		bs, err := os.ReadFile(relayerConfigPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bs, &awmRelayerConfig); err != nil {
			return err
		}
	} else {
		awmRelayerConfig = createRelayerConfig(
			logging.Info.LowerString(),
			relayerStorageDir,
			network.Endpoint,
		)
	}
	host, port, err := getURIHostAndPort(network.Endpoint)
	if err != nil {
		return err
	}
	addChainToRelayerConfig(
		&awmRelayerConfig,
		host,
		port,
		subnetID,
		blockchainID,
		teleporterContractAddress,
		teleporterRegistryAddress,
		relayerAddress,
		relayerPrivateKey,
	)
	bs, err := json.MarshalIndent(awmRelayerConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(relayerConfigPath, bs, constants.WriteReadReadPerms); err != nil {
		return err
	}
	return nil
}

func createRelayerConfig(
	logLevel string,
	storageLocation string,
	endpoint string,
) config.Config {
	return config.Config{
		LogLevel:               logLevel,
		PChainAPIURL:           endpoint,
		InfoAPIURL:             endpoint,
		StorageLocation:        storageLocation,
		ProcessMissedBlocks:    false,
		SourceBlockchains:      []*config.SourceBlockchain{},
		DestinationBlockchains: []*config.DestinationBlockchain{},
	}
}

func addChainToRelayerConfig(
	relayerConfig *config.Config,
	host string,
	port uint32,
	subnetID string,
	blockchainID string,
	teleporterContractAddress string,
	teleporterRegistryAddress string,
	relayerRewardAddress string,
	relayerFundedAddressKey string,
) {
	source := &config.SourceBlockchain{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
		VM:           config.EVM.String(),
		RPCEndpoint:  fmt.Sprintf("http://%s:%d/ext/bc/%s/rpc", host, port, blockchainID),
		WSEndpoint:   fmt.Sprintf("ws://%s:%d/ext/bc/%s/ws", host, port, blockchainID),
		MessageContracts: map[string]config.MessageProtocolConfig{
			teleporterContractAddress: {
				MessageFormat: config.TELEPORTER.String(),
				Settings: map[string]interface{}{
					"reward-address": relayerRewardAddress,
				},
			},
			offchainregistry.OffChainRegistrySourceAddress.Hex(): {
				MessageFormat: config.OFF_CHAIN_REGISTRY.String(),
				Settings: map[string]interface{}{
					"teleporter-registry-address": teleporterRegistryAddress,
				},
			},
		},
	}
	destination := &config.DestinationBlockchain{
		SubnetID:          subnetID,
		BlockchainID:      blockchainID,
		VM:                config.EVM.String(),
		RPCEndpoint:       fmt.Sprintf("http://%s:%d/ext/bc/%s/rpc", host, port, blockchainID),
		AccountPrivateKey: relayerFundedAddressKey,
	}
	if !utils.Any(relayerConfig.SourceBlockchains, func(s *config.SourceBlockchain) bool { return s.BlockchainID == blockchainID }) {
		relayerConfig.SourceBlockchains = append(relayerConfig.SourceBlockchains, source)
	}
	if !utils.Any(relayerConfig.DestinationBlockchains, func(s *config.DestinationBlockchain) bool { return s.BlockchainID == blockchainID }) {
		relayerConfig.DestinationBlockchains = append(relayerConfig.DestinationBlockchains, destination)
	}
}

// Get the host and port from a URI. The URI should be in the format http://host:port or https://host:port or host:port
func getURIHostAndPort(uri string) (string, uint32, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse uri %s: %w", uri, err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host/port at uri %s: %w", uri, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert port to uint at uri %s: %w", uri, err)
	}
	return host, uint32(port), nil
}

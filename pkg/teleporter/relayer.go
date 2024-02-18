// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/awm-relayer/config"
	offchainregistry "github.com/ava-labs/awm-relayer/messages/off-chain-registry"
)

const (
	teleporterRelayerPrivateKey = "C2CE4E001B7585F543982A01FBC537CFF261A672FA8BD1FAFC08A207098FE2DE"
	teleporterRelayerAddress    = "0xA100fF48a37cab9f87c8b5Da933DA46ea1a5fb80"
)

var (
	teleporterRelayerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX
)

func FundRelayer(
	rpcURL string,
	prefundedPrivateKey string,
) error {
	// get teleporter relayer balance
	teleporterRelayerBalance, err := evm.GetAddressBalance(rpcURL, teleporterRelayerAddress)
	if err != nil {
		return err
	}
	if teleporterRelayerBalance.Cmp(teleporterRelayerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(teleporterRelayerRequiredBalance, teleporterRelayerBalance)
		err := evm.FundAddress(
			rpcURL,
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
	version string,
	binDir string,
	configPath string,
	logFilePath string,
	runFilePath string,
	storageDir string,
) error {
	if err := relayerCleanup(runFilePath, storageDir); err != nil {
		return err
	}
	binPath, err := installRelayer(version, binDir)
	if err != nil {
		return err
	}
	pid, err := executeRelayer(binPath, configPath, logFilePath)
	if err != nil {
		return err
	}
	return saveRelayerRunFile(runFilePath, pid)
}

func relayerCleanup(runFilePath string, storageDir string) error {
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
		// so much expected after a reboot
		if err := os.Remove(runFilePath); err != nil {
			return fmt.Errorf("failed removing relayer run file %s: %w", runFilePath, err)
		}
		return nil
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed killing relayer process with pid %d: %w", rf.Pid, err)
	}
	if err := os.Remove(runFilePath); err != nil {
		return fmt.Errorf("failed removing relayer run file %s: %w", runFilePath, err)
	}
	return nil
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
		return fmt.Errorf("could not write awm relater run file to %s: %w", err)
	}
	return nil
}

func installRelayer(binDir, version string) (string, error) {
	binPath := filepath.Join(binDir, version, constants.AWMRelayerBin)
	if utils.IsExecutable(binPath) {
		ux.Logger.PrintToUser("AWM-Relayer %s is already installed", version)
		return binPath, nil
	}
	ux.Logger.PrintToUser("installing AWM-Relayer %s", version)
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
			network.ID,
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
		teleporterRelayerAddress,
		teleporterRelayerPrivateKey,
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
	networkID uint32,
	endpoint string,
) config.Config {
	return config.Config{
		LogLevel:            logLevel,
		NetworkID:           networkID,
		PChainAPIURL:        endpoint,
		EncryptConnection:   false,
		StorageLocation:     storageLocation,
		ProcessMissedBlocks: false,
		SourceSubnets:       []*config.SourceSubnet{},
		DestinationSubnets:  []*config.DestinationSubnet{},
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
	source := &config.SourceSubnet{
		SubnetID:          subnetID,
		BlockchainID:      blockchainID,
		VM:                config.EVM.String(),
		EncryptConnection: false,
		APINodeHost:       host,
		APINodePort:       port,
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
	destination := &config.DestinationSubnet{
		SubnetID:          subnetID,
		BlockchainID:      blockchainID,
		VM:                config.EVM.String(),
		EncryptConnection: false,
		APINodeHost:       host,
		APINodePort:       port,
		AccountPrivateKey: relayerFundedAddressKey,
	}
	sources := relayerConfig.SourceSubnets
	found := false
	for _, s := range sources {
		if s.BlockchainID == source.BlockchainID {
			found = true
		}
	}
	if !found {
		relayerConfig.SourceSubnets = append(sources, source)
	}
	destinations := relayerConfig.DestinationSubnets
	found = false
	for _, d := range destinations {
		if d.BlockchainID == destination.BlockchainID {
			found = true
		}
	}
	if !found {
		relayerConfig.DestinationSubnets = append(destinations, destination)
	}
}

// Get the host and port from a URI. The URI should be in the format http://host:port or https://host:port or host:port
func getURIHostAndPort(uri string) (string, uint32, error) {
	trimmedUri := uri
	trimmedUri = strings.TrimPrefix(trimmedUri, "http://")
	trimmedUri = strings.TrimPrefix(trimmedUri, "https://")
	hostAndPort := strings.Split(trimmedUri, ":")
	if len(hostAndPort) != 2 {
		return "", 0, fmt.Errorf("expected only host and port fields in %s", uri)
	}
	port, err := strconv.ParseUint(hostAndPort[1], 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port from %s: %w", uri, err)
	}
	return hostAndPort[0], uint32(port), nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.
package interchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	apiConfig "github.com/ava-labs/icm-services/config"
	offchainregistry "github.com/ava-labs/icm-services/messages/off-chain-registry"
	"github.com/ava-labs/icm-services/relayer/config"
)

const (
	localRelayerSetupTime         = 2 * time.Second
	localRelayerCheckPoolTime     = 100 * time.Millisecond
	localRelayerCheckTimeout      = 3 * time.Second
	defaultDBWriteIntervalSeconds = 10
	defaultSignatureCacheSize     = 1024 * 1024
)

var relayerRequiredBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(500)) // 500 AVAX

func GetRelayerKeyInfo(keyPath string) (string, string, error) {
	var (
		k   *key.SoftKey
		err error
	)
	if utils.FileExists(keyPath) {
		k, err = key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		if err != nil {
			return "", "", err
		}
	} else {
		k, err = key.NewSoft(0)
		if err != nil {
			return "", "", err
		}
		if err := k.Save(keyPath); err != nil {
			return "", "", err
		}
	}
	return k.C(), k.PrivKeyHex(), nil
}

func FundRelayer(
	rpcURL string,
	prefundedPrivateKey string,
	relayerAddress string,
) error {
	// get relayer balance
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	relayerBalance, err := evm.GetAddressBalance(client, relayerAddress)
	if err != nil {
		return err
	}
	if relayerBalance.Cmp(relayerRequiredBalance) < 0 {
		toFund := big.NewInt(0).Sub(relayerRequiredBalance, relayerBalance)
		err := evm.FundAddress(
			client,
			prefundedPrivateKey,
			relayerAddress,
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
	binPath string,
	binDir string,
	configPath string,
	logFilePath string,
	runFilePath string,
	storageDir string,
) (string, error) {
	if err := RelayerCleanup(runFilePath, logFilePath, storageDir); err != nil {
		return "", err
	}
	if binPath == "" {
		var err error
		binPath, err = InstallRelayer(binDir, version)
		if err != nil {
			return "", err
		}
	}
	pid, err := executeRelayer(binPath, configPath, logFilePath)
	if err != nil {
		return "", err
	}
	return binPath, saveRelayerRunFile(runFilePath, pid)
}

func RelayerIsUp(runFilePath string) (bool, int, *os.Process, error) {
	if !utils.FileExists(runFilePath) {
		return false, 0, nil, nil
	}
	bs, err := os.ReadFile(runFilePath)
	if err != nil {
		return false, 0, nil, err
	}
	rf := relayerRunFile{}
	if err := json.Unmarshal(bs, &rf); err != nil {
		return false, 0, nil, err
	}
	proc, err := utils.GetProcess(rf.Pid)
	if err != nil {
		// after a reboot without network cleanup, it is expected that the file pid will exist but the process not
		return false, 0, nil, removeRelayerRunFile(runFilePath)
	}
	return true, rf.Pid, proc, nil
}

func RelayerCleanup(
	runFilePath string,
	logFilePath string,
	storageDir string,
) error {
	_ = os.Remove(logFilePath)
	if err := os.RemoveAll(storageDir); err != nil {
		return err
	}
	relayerIsUp, pid, proc, err := RelayerIsUp(runFilePath)
	if err != nil {
		return err
	}
	if relayerIsUp {
		waitedCh := make(chan struct{})
		go func() {
			for {
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					if errors.Is(err, os.ErrProcessDone) {
						close(waitedCh)
						return
					} else {
						ux.Logger.RedXToUser("failure checking to process pid %d aliveness due to: %s", proc.Pid, err)
					}
				}
				time.Sleep(localRelayerCheckPoolTime)
			}
		}()
		if err := proc.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("failed sending interrupt signal to relayer process with pid %d: %w", pid, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), localRelayerCheckTimeout)
		defer cancel()
		select {
		case <-ctx.Done():
			if err := proc.Signal(os.Kill); err != nil {
				return fmt.Errorf("failed killing relayer process with pid %d: %w", pid, err)
			}
		case <-waitedCh:
		}
		return removeRelayerRunFile(runFilePath)
	}
	return nil
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
	if err := os.MkdirAll(filepath.Dir(runFilePath), constants.DefaultPerms755); err != nil {
		return err
	}
	if err := os.WriteFile(runFilePath, bs, constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("could not write awm relater run file to %s: %w", runFilePath, err)
	}
	return nil
}

func GetLatestRelayerReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.ICMRelayerKind,
	)
}

func GetLatestRelayerPreReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.ICMRelayerKind,
	)
}

func InstallRelayer(binDir, version string) (string, error) {
	if version == "" || version == constants.LatestPreReleaseVersionTag {
		var err error
		version, err = GetLatestRelayerPreReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	if version == constants.LatestReleaseVersionTag {
		var err error
		version, err = GetLatestRelayerReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Relayer version %s", version)
	versionBinDir := filepath.Join(binDir, version)
	binPath := filepath.Join(versionBinDir, constants.ICMRelayerBin)
	if utils.IsExecutable(binPath) {
		return binPath, nil
	}
	ux.Logger.PrintToUser("Installing Relayer")
	url, err := getRelayerURL(version)
	if err != nil {
		return "", err
	}
	bs, err := utils.Download(url)
	if err != nil {
		return "", err
	}
	if err := binutils.InstallArchive("tar.gz", bs, versionBinDir); err != nil {
		return "", err
	}
	return binPath, nil
}

func executeRelayer(binPath string, configPath string, logFile string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logFile), constants.DefaultPerms755); err != nil {
		return 0, err
	}
	logWriter, err := os.Create(logFile)
	if err != nil {
		return 0, err
	}

	ux.Logger.PrintToUser("Executing Relayer")

	cmd := exec.Command(binPath, "--config-file", configPath)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	ch := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		ch <- struct{}{}
	}()
	time.Sleep(localRelayerSetupTime)
	select {
	case <-ch:
		return 0, fmt.Errorf("relayer process failed during setup")
	default:
	}

	err = waitForRelayerInitialization(
		configPath,
		logFile,
		0,
		0,
	)

	return cmd.Process.Pid, err
}

func getRelayerURL(version string) (string, error) {
	goarch, goos := runtime.GOARCH, runtime.GOOS
	if goos != "linux" && goos != "darwin" {
		return "", fmt.Errorf("OS not supported: %s", goos)
	}
	component := "icm-relayer"
	semanticVersion := strings.TrimPrefix(version, component+"/")
	if semanticVersion != version {
		return fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/icm-relayer%%2F%s/icm-relayer_%s_%s_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.ICMServicesRepoName,
			semanticVersion,
			strings.TrimPrefix(semanticVersion, "v"),
			goos,
			goarch,
		), nil
	}
	semanticVersion = strings.TrimPrefix(version, component+"-")
	if semanticVersion != version {
		return fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/icm-relayer-%s/icm-relayer_%s_%s_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.ICMServicesRepoName,
			semanticVersion,
			strings.TrimPrefix(semanticVersion, "v"),
			goos,
			goarch,
		), nil
	}
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/icm-relayer_%s_%s_%s.tar.gz",
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		semanticVersion,
		strings.TrimPrefix(semanticVersion, "v"),
		goos,
		goarch,
	), nil
}

func loadRelayerConfig(relayerConfigPath string) (*config.Config, error) {
	awmRelayerConfig := config.Config{}
	bs, err := os.ReadFile(relayerConfigPath)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bs, &awmRelayerConfig); err != nil {
		return nil, err
	}
	return &awmRelayerConfig, nil
}

func saveRelayerConfig(relayerConfig *config.Config, relayerConfigPath string) error {
	if err := os.MkdirAll(filepath.Dir(relayerConfigPath), constants.DefaultPerms755); err != nil {
		return err
	}
	bs, err := json.MarshalIndent(relayerConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(relayerConfigPath, bs, constants.WriteReadReadPerms)
}

func CreateBaseRelayerConfigIfMissing(
	relayerConfigPath string,
	logLevel string,
	storageLocation string,
	metricsPort uint16,
	network models.Network,
	allowPrivateIPs bool,
) error {
	if !utils.FileExists(relayerConfigPath) {
		return CreateBaseRelayerConfig(
			relayerConfigPath,
			logLevel,
			storageLocation,
			metricsPort,
			network,
			allowPrivateIPs,
		)
	}
	return nil
}

func CreateBaseRelayerConfig(
	relayerConfigPath string,
	logLevel string,
	storageLocation string,
	metricsPort uint16,
	network models.Network,
	allowPrivateIPs bool,
) error {
	awmRelayerConfig := &config.Config{
		LogLevel: logLevel,
		PChainAPI: &apiConfig.APIConfig{
			BaseURL:     network.Endpoint,
			QueryParams: map[string]string{},
		},
		InfoAPI: &apiConfig.APIConfig{
			BaseURL:     network.Endpoint,
			QueryParams: map[string]string{},
		},
		StorageLocation:        storageLocation,
		ProcessMissedBlocks:    false,
		SourceBlockchains:      []*config.SourceBlockchain{},
		DestinationBlockchains: []*config.DestinationBlockchain{},
		MetricsPort:            metricsPort,
		DBWriteIntervalSeconds: defaultDBWriteIntervalSeconds,
		SignatureCacheSize:     defaultSignatureCacheSize,
		AllowPrivateIPs:        allowPrivateIPs,
	}
	return saveRelayerConfig(awmRelayerConfig, relayerConfigPath)
}

func AddSourceAndDestinationToRelayerConfig(
	relayerConfigPath string,
	rpcEndpoint string,
	wsEndpoint string,
	subnetID string,
	blockchainID string,
	icmRegistryAddress string,
	icmMessengerAddress string,
	relayerRewardAddress string,
	relayerPrivateKey string,
) error {
	awmRelayerConfig, err := loadRelayerConfig(relayerConfigPath)
	if err != nil {
		return err
	}
	addSourceToRelayerConfig(
		awmRelayerConfig,
		rpcEndpoint,
		wsEndpoint,
		subnetID,
		blockchainID,
		icmRegistryAddress,
		icmMessengerAddress,
		relayerRewardAddress,
	)
	addDestinationToRelayerConfig(
		awmRelayerConfig,
		rpcEndpoint,
		subnetID,
		blockchainID,
		relayerPrivateKey,
	)
	return saveRelayerConfig(awmRelayerConfig, relayerConfigPath)
}

func AddSourceToRelayerConfig(
	relayerConfigPath string,
	rpcEndpoint string,
	wsEndpoint string,
	subnetID string,
	blockchainID string,
	icmRegistryAddress string,
	icmMessengerAddress string,
	relayerRewardAddress string,
) error {
	awmRelayerConfig, err := loadRelayerConfig(relayerConfigPath)
	if err != nil {
		return err
	}
	addSourceToRelayerConfig(
		awmRelayerConfig,
		rpcEndpoint,
		wsEndpoint,
		subnetID,
		blockchainID,
		icmRegistryAddress,
		icmMessengerAddress,
		relayerRewardAddress,
	)
	return saveRelayerConfig(awmRelayerConfig, relayerConfigPath)
}

func AddDestinationToRelayerConfig(
	relayerConfigPath string,
	rpcEndpoint string,
	subnetID string,
	blockchainID string,
	relayerPrivateKey string,
) error {
	awmRelayerConfig, err := loadRelayerConfig(relayerConfigPath)
	if err != nil {
		return err
	}
	addDestinationToRelayerConfig(
		awmRelayerConfig,
		rpcEndpoint,
		subnetID,
		blockchainID,
		relayerPrivateKey,
	)
	return saveRelayerConfig(awmRelayerConfig, relayerConfigPath)
}

func addSourceToRelayerConfig(
	relayerConfig *config.Config,
	rpcEndpoint string,
	wsEndpoint string,
	subnetID string,
	blockchainID string,
	icmRegistryAddress string,
	icmMessengerAddress string,
	relayerRewardAddress string,
) {
	if wsEndpoint == "" {
		wsEndpoint = strings.TrimPrefix(rpcEndpoint, "https")
		wsEndpoint = strings.TrimPrefix(wsEndpoint, "http")
		wsEndpoint = strings.TrimSuffix(wsEndpoint, "rpc")
		wsEndpoint = fmt.Sprintf("%s%s%s", "ws", wsEndpoint, "ws")
	}
	source := &config.SourceBlockchain{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
		VM:           config.EVM.String(),
		RPCEndpoint: apiConfig.APIConfig{
			BaseURL: rpcEndpoint,
		},
		WSEndpoint: apiConfig.APIConfig{
			BaseURL: wsEndpoint,
		},
		MessageContracts: map[string]config.MessageProtocolConfig{
			icmMessengerAddress: {
				MessageFormat: config.TELEPORTER.String(),
				Settings: map[string]interface{}{
					"reward-address": relayerRewardAddress,
				},
			},
			offchainregistry.OffChainRegistrySourceAddress.Hex(): {
				MessageFormat: config.OFF_CHAIN_REGISTRY.String(),
				Settings: map[string]interface{}{
					"teleporter-registry-address": icmRegistryAddress,
				},
			},
		},
	}
	if !utils.Any(relayerConfig.SourceBlockchains, func(s *config.SourceBlockchain) bool { return s.BlockchainID == blockchainID }) {
		relayerConfig.SourceBlockchains = append(relayerConfig.SourceBlockchains, source)
	}
}

func addDestinationToRelayerConfig(
	relayerConfig *config.Config,
	rpcEndpoint string,
	subnetID string,
	blockchainID string,
	relayerFundedAddressKey string,
) {
	destination := &config.DestinationBlockchain{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
		VM:           config.EVM.String(),
		RPCEndpoint: apiConfig.APIConfig{
			BaseURL: rpcEndpoint,
		},
		AccountPrivateKey: relayerFundedAddressKey,
	}
	if !utils.Any(relayerConfig.DestinationBlockchains, func(s *config.DestinationBlockchain) bool { return s.BlockchainID == blockchainID }) {
		relayerConfig.DestinationBlockchains = append(relayerConfig.DestinationBlockchains, destination)
	}
}

func waitForRelayerInitialization(
	relayerConfigPath string,
	logPath string,
	checkInterval time.Duration,
	checkTimeout time.Duration,
) error {
	config, err := loadRelayerConfig(relayerConfigPath)
	if err != nil {
		return err
	}
	sourceBlockchains := []string{}
	for _, source := range config.SourceBlockchains {
		sourceBlockchains = append(sourceBlockchains, source.BlockchainID)
	}
	if checkInterval == 0 {
		checkInterval = 100 * time.Millisecond
	}
	if checkTimeout == 0 {
		checkTimeout = 120 * time.Second
	}
	t0 := time.Now()
	for {
		bs, err := os.ReadFile(logPath)
		if err != nil {
			return err
		}
		sourcesInitialized := 0
		for _, l := range strings.Split(string(bs), "\n") {
			for _, sourceBlockchain := range sourceBlockchains {
				if strings.Contains(l, "Listener initialized") && strings.Contains(l, sourceBlockchain) {
					sourcesInitialized++
				}
			}
		}
		if sourcesInitialized == len(sourceBlockchains) {
			break
		}
		elapsed := time.Since(t0)
		if elapsed > checkTimeout {
			return fmt.Errorf("timeout waiting for relayer initialization")
		}
		time.Sleep(checkInterval)
	}
	return nil
}

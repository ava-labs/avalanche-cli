// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureaggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/api/info"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func NewSignatureAggregatorLogger(
	aggregatorLogLevel string,
	aggregatorLogToStdout bool,
	logDir string,
) (logging.Logger, error) {
	return utils.NewLogger(
		constants.SignatureAggregator,
		aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		logDir,
		aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
}

func GetLatestSignatureAggregatorReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.SignatureAggregator,
	)
}

func GetLatestSignatureAggregatorPreReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.SignatureAggregator,
	)
}

func InstallSignatureAggregator(app *application.Avalanche, version string) (string, error) {
	if version == "" || version == constants.LatestPreReleaseVersionTag {
		var err error
		version, err = GetLatestSignatureAggregatorPreReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	if version == constants.LatestReleaseVersionTag {
		var err error
		version, err = GetLatestSignatureAggregatorReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Signature Aggregator version %s", version)
	versionBinDir := filepath.Join(app.GetSignatureAggregatorBinDir(), version)
	binPath := filepath.Join(versionBinDir, constants.SignatureAggregator)
	if utils.IsExecutable(binPath) {
		return binPath, nil
	}
	ux.Logger.PrintToUser("Installing Signature Aggregator")
	url, err := getSignatureAggregatorURL(version)
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

func getSignatureAggregatorURL(version string) (string, error) {
	goarch, goos := runtime.GOARCH, runtime.GOOS
	if goos != "linux" && goos != "darwin" {
		return "", fmt.Errorf("OS not supported: %s", goos)
	}
	component := "signature-aggregator"
	semanticVersion := strings.TrimPrefix(version, component+"/")
	if semanticVersion != version {
		return fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/signature-aggregator%%2F%s/signature-aggregator_%s_%s_%s.tar.gz",
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
			"https://github.com/%s/%s/releases/download/signature-aggregator-%s/signature-aggregator_%s_%s_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.ICMServicesRepoName,
			semanticVersion,
			strings.TrimPrefix(semanticVersion, "v"),
			goos,
			goarch,
		), nil
	}
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/signature-aggregator_%s_%s_%s.tar.gz",
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		semanticVersion,
		strings.TrimPrefix(semanticVersion, "v"),
		goos,
		goarch,
	), nil
}

// StartSignatureAggregator starts the signature aggregator process.
// It handles port conflicts and retries if necessary.
func StartSignatureAggregator(app *application.Avalanche, configPath string, logFile string, logger logging.Logger, version string, signatureAggregatorEndpoint string) (int, error) {
	binPath, err := InstallSignatureAggregator(app, version)
	if err != nil {
		return 0, err
	}
	fmt.Printf("binPath %s \n", binPath)

	// TODO: remove kill process below once cleanup is implemented
	//Function to check if port is in use
	isPortInUse := func() bool {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err == nil {
			if err := conn.Close(); err != nil {
				logger.Warn("Failed to close connection while checking port",
					zap.Error(err),
				)
			}
			return true
		}
		return false
	}

	// Function to kill existing process
	killExistingProcess := func() error {
		// Try pkill first
		cmd := exec.Command("pkill", "-f", "signature-aggregator")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to kill existing signature-aggregator process: %w", err)
		}
		return nil
	}

	// Try to start the aggregator with retries
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if isPortInUse() {
			logger.Info("Port 8080 is in use, attempting to kill existing process",
				zap.Int("attempt", i+1),
				zap.Int("max_attempts", maxRetries),
			)
			if err := killExistingProcess(); err != nil {
				logger.Warn("Failed to kill existing process",
					zap.Error(err),
				)
			}
			// Wait for port to be released
			time.Sleep(2 * time.Second)
		}

		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			return 0, err
		}

		logWriter, err := os.Create(logFile)
		if err != nil {
			return 0, err
		}

		logger.Info("Starting Signature Aggregator",
			zap.Int("attempt", i+1),
			zap.Int("max_attempts", maxRetries),
		)

		cmd := exec.Command(binPath, "--config-file", configPath)
		cmd.Stdout = logWriter
		cmd.Stderr = logWriter
		fmt.Printf("cmd %s \n", cmd.Path)
		if err := cmd.Start(); err != nil {
			if closeErr := logWriter.Close(); closeErr != nil {
				return 0, closeErr
			}
			if i == maxRetries-1 {
				return 0, fmt.Errorf("failed to start signature-aggregator after %d attempts: %w", maxRetries, err)
			}
			continue
		}

		ch := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			ch <- struct{}{}
		}()

		// Allow time for the aggregator to initialize
		time.Sleep(2 * time.Second)

		select {
		case <-ch:
			if i == maxRetries-1 {
				return 0, fmt.Errorf("signature-aggregator exited during startup after %d attempts", maxRetries)
			}
			continue
		default:
		}

		// Check if the aggregator is ready
		//if err := waitForAggregatorReady("http://localhost:8080/aggregate-signatures", 5*time.Second); err != nil {
		fmt.Printf("waiting for %s \n", signatureAggregatorEndpoint)
		if err := waitForAggregatorReady(signatureAggregatorEndpoint, 5*time.Second); err != nil {
			_ = cmd.Process.Kill()
			if i == maxRetries-1 {
				return 0, fmt.Errorf("signature-aggregator not ready after %d attempts: %w", maxRetries, err)
			}
			continue
		}

		logger.Info("Signature Aggregator started successfully",
			zap.Int("attempt", i+1),
		)
		return cmd.Process.Pid, nil
	}

	return 0, fmt.Errorf("failed to start signature-aggregator after %d attempts", maxRetries)
}

func waitForAggregatorReady(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for signature-aggregator readiness")
		case <-ticker.C:
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("obtained error %s \n", err)
			}
			if err == nil && resp.StatusCode == http.StatusBadRequest {
				// A 400 means the service is up but received a malformed request
				if closeErr := resp.Body.Close(); closeErr != nil {
					fmt.Printf("Failed to close response body: %v\n", closeErr)
				}
				return nil
			}
		}
	}
}

// readExistingConfig reads the existing signature aggregator configuration from a file.
func readExistingConfig(configPath string) (*SignatureAggregatorConfig, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}

	// Read the file
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the config
	var config SignatureAggregatorConfig
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func CreateSignatureAggregatorConfig(subnetID string, networkEndpoint string, peers []info.Peer, apiPort, metricsPort int) *SignatureAggregatorConfig {
	config := &SignatureAggregatorConfig{
		LogLevel:             "debug",
		PChainAPI:            APIConfig{BaseURL: networkEndpoint},
		InfoAPI:              APIConfig{BaseURL: networkEndpoint},
		SignatureCacheSize:   1048576,
		AllowPrivateIPs:      true,
		TrackedSubnetIDs:     []string{subnetID},
		ManuallyTrackedPeers: make([]PeerConfig, 0),
		APIPort:              apiPort,
		MetricsPort:          metricsPort,
	}

	for _, peer := range peers {
		// Skip peers with invalid IP addresses
		if !peer.Info.PublicIP.IsValid() {
			continue
		}
		config.ManuallyTrackedPeers = append(config.ManuallyTrackedPeers, PeerConfig{
			ID: peer.Info.ID.String(),
			IP: peer.Info.PublicIP.String(),
		})
	}

	return config
}

// WriteSignatureAggregatorConfig writes the signature aggregator configuration to a file.
func WriteSignatureAggregatorConfig(config *SignatureAggregatorConfig, configPath string) error {
	fmt.Printf("config path %s \n", configPath)
	// Read existing config if it exists
	existingConfig, err := readExistingConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read existing config: %w", err)
	}

	// If existing config exists, merge with it
	if existingConfig != nil {
		// Merge tracked subnet IDs
		existingSubnetIDs := make(map[string]struct{})
		for _, id := range existingConfig.TrackedSubnetIDs {
			existingSubnetIDs[id] = struct{}{}
		}
		for _, id := range config.TrackedSubnetIDs {
			existingSubnetIDs[id] = struct{}{}
		}
		mergedSubnetIDs := make([]string, 0, len(existingSubnetIDs))
		for id := range existingSubnetIDs {
			mergedSubnetIDs = append(mergedSubnetIDs, id)
		}
		config.TrackedSubnetIDs = mergedSubnetIDs

		// Merge manually tracked peers
		existingPeers := make(map[string]PeerConfig)
		for _, peer := range existingConfig.ManuallyTrackedPeers {
			existingPeers[peer.ID] = peer
		}
		for _, peer := range config.ManuallyTrackedPeers {
			existingPeers[peer.ID] = peer
		}
		mergedPeers := make([]PeerConfig, 0, len(existingPeers))
		for _, peer := range existingPeers {
			mergedPeers = append(mergedPeers, peer)
		}
		config.ManuallyTrackedPeers = mergedPeers

		// Keep other existing values
		config.LogLevel = existingConfig.LogLevel
		config.SignatureCacheSize = existingConfig.SignatureCacheSize
		config.AllowPrivateIPs = existingConfig.AllowPrivateIPs
	}

	// Marshal and write the config
	configBytes, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SignatureAggregatorConfig represents the configuration for the signature aggregator.
type SignatureAggregatorConfig struct {
	LogLevel             string       `json:"log-level"`
	PChainAPI            APIConfig    `json:"p-chain-api"`
	InfoAPI              APIConfig    `json:"info-api"`
	SignatureCacheSize   int          `json:"signature-cache-size"`
	AllowPrivateIPs      bool         `json:"allow-private-ips"`
	TrackedSubnetIDs     []string     `json:"tracked-subnet-ids"`
	ManuallyTrackedPeers []PeerConfig `json:"manually-tracked-peers"`
	APIPort              int          `json:"api-port"`
	MetricsPort          int          `json:"metrics-port"`
}

// APIConfig represents the configuration for an API endpoint.
type APIConfig struct {
	BaseURL string `json:"base-url"`
}

// PeerConfig represents the configuration for a peer.
type PeerConfig struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

func CreateSignatureAggregatorInstance(app *application.Avalanche, subnetIDStr string, network models.Network, extraAggregatorPeers []info.Peer, aggregatorLogger logging.Logger, version string) error {
	// Create config file for signature aggregator
	//apiPort, metricsPort, err := generateAPIMetricsPorts()
	//if err != nil {
	//	return fmt.Errorf("failed to generate api and metrics ports: %w", err)
	//}
	apiPort := 8080
	metricsPort := 8081
	config := CreateSignatureAggregatorConfig(subnetIDStr, network.Endpoint, extraAggregatorPeers, apiPort, metricsPort)
	//configPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind, subnetIDStr), "config.json")
	configPath := filepath.Join(app.GetSignatureAggregatorBinDir(), "config.json")
	if err := WriteSignatureAggregatorConfig(config, configPath); err != nil {
		return fmt.Errorf("failed to write signature aggregator config: %w", err)
	}
	//logPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind, subnetIDStr), "signature-aggregator.log")
	logPath := filepath.Join(app.GetSignatureAggregatorBinDir(), "signature-aggregator.log")
	signatureAggregatorEndpoint := fmt.Sprintf("http://localhost:%d/aggregate-signatures", apiPort)
	fmt.Printf("signatureAggregatorEndpoint %s \n", signatureAggregatorEndpoint)
	pid, err := StartSignatureAggregator(app, configPath, logPath, aggregatorLogger, version, signatureAggregatorEndpoint)
	if err != nil {
		return fmt.Errorf("failed to start signature aggregator: %w", err)
	}
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind, subnetIDStr)

	return saveSignatureAggregatorFile(runFilePath, pid, apiPort, metricsPort)
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		// If we can't connect, the port is available
		return true
	}
	// If we can connect, the port is in use
	conn.Close()
	return false
}

func generateAPIMetricsPorts() (int, int, error) {
	// Create a context with a 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start with default ports and increment by 2 each time
	apiPort := 8080
	metricsPort := 8081

	// Keep trying until we find available ports or timeout
	for {
		select {
		case <-ctx.Done():
			return 0, 0, fmt.Errorf("timeout while searching for available ports: %w", ctx.Err())
		default:
			if isPortAvailable(apiPort) && isPortAvailable(metricsPort) {
				fmt.Printf("both ports are availble %s and %s \n", apiPort, metricsPort)
				return apiPort, metricsPort, nil
			}
			fmt.Printf("trying ports apiPort %s and metrics Port %s \n", apiPort, metricsPort)
			apiPort += 2
			metricsPort += 2
		}
	}
}

func GetSignatureAggregatorEndpoint() (string, error) {
	return "http://localhost:8080/aggregate-signatures", nil
}

type signatureAggregatorRunFile struct {
	Pid         int `json:"pid"`
	APIPort     int `json:"api_port"`
	MetricsPort int `json:"metrics_port"`
}

func saveSignatureAggregatorFile(runFilePath string, pid, apiPort, metricsPort int) error {
	rf := signatureAggregatorRunFile{
		Pid:         pid,
		APIPort:     apiPort,
		MetricsPort: metricsPort,
	}
	bs, err := json.Marshal(&rf)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(runFilePath), constants.DefaultPerms755); err != nil {
		return err
	}
	if err := os.WriteFile(runFilePath, bs, constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("could not write signature aggregator run file to %s: %w", runFilePath, err)
	}
	return nil
}

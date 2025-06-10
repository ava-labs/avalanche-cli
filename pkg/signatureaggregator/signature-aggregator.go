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

func InstallSignatureAggregator(app *application.Avalanche, version *string) (string, error) {
	if *version == "" || *version == constants.LatestPreReleaseVersionTag {
		var err error
		*version, err = GetLatestSignatureAggregatorPreReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	if *version == constants.LatestReleaseVersionTag {
		var err error
		*version, err = GetLatestSignatureAggregatorReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Signature Aggregator version %s", *version)
	versionBinDir := filepath.Join(app.GetSignatureAggregatorBinDir(), *version)
	binPath := filepath.Join(versionBinDir, constants.SignatureAggregator)
	if utils.IsExecutable(binPath) {
		return binPath, nil
	}
	ux.Logger.PrintToUser("Installing Signature Aggregator")
	url, err := getSignatureAggregatorURL(*version)
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
func StartSignatureAggregator(app *application.Avalanche, network models.Network, configPath string, logFile string, logger logging.Logger, version string, signatureAggregatorEndpoint string) (int, error) {
	binPath, err := InstallSignatureAggregator(app, &version)
	if err != nil {
		return 0, err
	}
	fmt.Printf("binPath %s \n", binPath)

	// Stop any existing signature aggregator process
	if err := stopSignatureAggregator(app, network); err != nil {
		logger.Warn("Failed to stop existing signature aggregator",
			zap.Error(err),
		)
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return 0, err
	}

	logWriter, err := os.Create(logFile)
	if err != nil {
		return 0, err
	}

	logger.Info("Starting Signature Aggregator")

	cmd := exec.Command(binPath, "--config-file", configPath)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	fmt.Printf("cmd %s \n", cmd.Path)
	if err := cmd.Start(); err != nil {
		if closeErr := logWriter.Close(); closeErr != nil {
			return 0, closeErr
		}
		return 0, fmt.Errorf("failed to start signature-aggregator: %w", err)
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
		return 0, fmt.Errorf("signature-aggregator exited during startup")
	default:
	}

	// Check if the aggregator is ready
	fmt.Printf("waiting for %s \n", signatureAggregatorEndpoint)
	if err := waitForAggregatorReady(signatureAggregatorEndpoint, 5*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("signature-aggregator not ready: %w", err)
	}

	logger.Info("Signature Aggregator started successfully")
	return cmd.Process.Pid, nil
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

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
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

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		// If we can't connect, the port is available
		return true
	}
	// If we can connect, the port is in use
	if err := conn.Close(); err != nil {
		fmt.Printf("Failed to close connection while checking port availability: %v\n", err)
	}
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
				return apiPort, metricsPort, nil
			}
			apiPort += 2
			metricsPort += 2
		}
	}
}

func CreateSignatureAggregatorInstance(app *application.Avalanche, subnetIDStr string, network models.Network, extraAggregatorPeers []info.Peer, aggregatorLogger logging.Logger, version string) error {
	// Create config file for signature aggregator
	var apiPort, metricsPort int
	var err error
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	// Check if run file exists and read ports from it
	if _, err := os.Stat(runFilePath); err == nil {
		// File exists, get process details
		runFile, err := getCurrentSignatureAggregatorProcessDetails(app, network)
		if err != nil {
			return fmt.Errorf("failed to get process details: %w", err)
		}
		// Use existing ports
		apiPort = runFile.APIPort
		metricsPort = runFile.MetricsPort
	} else {
		// Run file doesn't exist, generate new ports
		apiPort, metricsPort, err = generateAPIMetricsPorts()
		if err != nil {
			return fmt.Errorf("failed to generate api and metrics ports: %w", err)
		}
	}

	config := CreateSignatureAggregatorConfig(subnetIDStr, network.Endpoint, extraAggregatorPeers, apiPort, metricsPort)
	configPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind), "config.json")
	if err := WriteSignatureAggregatorConfig(config, configPath); err != nil {
		return fmt.Errorf("failed to write signature aggregator config: %w", err)
	}
	logPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind), "signature-aggregator.log")
	signatureAggregatorEndpoint := fmt.Sprintf("http://localhost:%d/aggregate-signatures", apiPort)
	fmt.Printf("signatureAggregatorEndpoint %s \n", signatureAggregatorEndpoint)
	pid, err := StartSignatureAggregator(app, network, configPath, logPath, aggregatorLogger, version, signatureAggregatorEndpoint)
	if err != nil {
		return fmt.Errorf("failed to start signature aggregator: %w", err)
	}

	return saveSignatureAggregatorFile(runFilePath, pid, apiPort, metricsPort, version)
}

func GetSignatureAggregatorEndpoint(app *application.Avalanche, network models.Network) (string, error) {
	runFile, err := getCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return "", fmt.Errorf("failed to get process details: %w", err)
	}
	fmt.Printf("obtained GetSignatureAggregatorEndpoint %s \n", fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort))
	return fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort), nil
}

type signatureAggregatorRunFile struct {
	Pid         int    `json:"pid"`
	APIPort     int    `json:"api_port"`
	MetricsPort int    `json:"metrics_port"`
	Version     string `json:"version"`
}

func saveSignatureAggregatorFile(runFilePath string, pid, apiPort, metricsPort int, version string) error {
	rf := signatureAggregatorRunFile{
		Pid:         pid,
		APIPort:     apiPort,
		MetricsPort: metricsPort,
		Version:     version,
	}
	bs, err := json.Marshal(&rf)
	if err != nil {
		return err
	}
	fmt.Printf("saveSignatureAggregatorFile %s \n", filepath.Dir(runFilePath))
	if err := os.MkdirAll(filepath.Dir(runFilePath), constants.DefaultPerms755); err != nil {
		return err
	}
	if err := os.WriteFile(runFilePath, bs, constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("could not write signature aggregator run file to %s: %w", runFilePath, err)
	}
	return nil
}

// getCurrentSignatureAggregatorProcessDetails reads the run file and returns the current process details.
// It returns the run file information including PID, ports, and version.
func getCurrentSignatureAggregatorProcessDetails(app *application.Avalanche, network models.Network) (*signatureAggregatorRunFile, error) {
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	runFileBytes, err := os.ReadFile(runFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read run file: %w", err)
	}

	var runFile signatureAggregatorRunFile
	if err := json.Unmarshal(runFileBytes, &runFile); err != nil {
		return nil, fmt.Errorf("failed to parse run file: %w", err)
	}

	return &runFile, nil
}

// stopSignatureAggregator stops the running signature aggregator process.
// It reads the run file to get the PID and kills the process if it's running.
func stopSignatureAggregator(app *application.Avalanche, network models.Network) error {
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	if _, err := os.Stat(runFilePath); os.IsNotExist(err) {
		return nil
	}

	runFile, err := getCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return fmt.Errorf("failed to get process details: %w", err)
	}

	// Kill existing process if running
	if runFile.Pid > 0 {
		process, err := os.FindProcess(runFile.Pid)
		if err == nil {
			if err := process.Kill(); err != nil {
				fmt.Printf("Failed to kill process %d: %v\n", runFile.Pid, err)
			}
		}
	}

	// Wait a bit for the process to be killed
	time.Sleep(2 * time.Second)
	return nil
}

// restartSignatureAggregator restarts the signature aggregator with the given config.
// It reads the run file to get the current ports and version, kills the existing process,
// and starts a new one with the updated config.
func restartSignatureAggregator(app *application.Avalanche, network models.Network, configPath string, logger logging.Logger) error {
	// Stop the existing signature aggregator
	if err := stopSignatureAggregator(app, network); err != nil {
		return fmt.Errorf("failed to stop signature aggregator: %w", err)
	}

	// Get current process details
	runFile, err := getCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return fmt.Errorf("failed to get process details: %w", err)
	}

	// Restart signature aggregator with updated config
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	logPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind), "signature-aggregator.log")
	signatureAggregatorEndpoint := fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort)
	pid, err := StartSignatureAggregator(app, network, configPath, logPath, logger, runFile.Version, signatureAggregatorEndpoint)
	if err != nil {
		return fmt.Errorf("failed to restart signature aggregator: %w", err)
	}

	// Update run file with new PID
	return saveSignatureAggregatorFile(runFilePath, pid, runFile.APIPort, runFile.MetricsPort, runFile.Version)
}

// UpdateSignatureAggregatorPeers updates the existing signature aggregator config with new peers.
// If new peers are found, it updates the config and restarts the signature aggregator.
func UpdateSignatureAggregatorPeers(app *application.Avalanche, network models.Network, extraAggregatorPeers []info.Peer, logger logging.Logger) error {
	// Get the config path
	configPath := filepath.Join(app.GetLocalSignatureAggregatorRunPath(network.Kind), "config.json")

	// Read existing config
	existingConfig, err := readExistingConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read existing config: %w", err)
	}
	if existingConfig == nil {
		return fmt.Errorf("no existing config found at %s", configPath)
	}

	// Convert existing peers to a map for easy lookup
	existingPeers := make(map[string]PeerConfig)
	for _, peer := range existingConfig.ManuallyTrackedPeers {
		existingPeers[peer.ID] = peer
	}

	// Check for new peers
	hasNewPeers := false
	for _, peer := range extraAggregatorPeers {
		if !peer.Info.PublicIP.IsValid() {
			continue
		}
		peerID := peer.Info.ID.String()
		if _, exists := existingPeers[peerID]; !exists {
			hasNewPeers = true
			existingConfig.ManuallyTrackedPeers = append(existingConfig.ManuallyTrackedPeers, PeerConfig{
				ID: peerID,
				IP: peer.Info.PublicIP.String(),
			})
		}
	}

	// If no new peers, no need to update
	if !hasNewPeers {
		return nil
	}

	// Write updated config
	if err := WriteSignatureAggregatorConfig(existingConfig, configPath); err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	// Restart the signature aggregator with the updated config
	return restartSignatureAggregator(app, network, configPath, logger)
}

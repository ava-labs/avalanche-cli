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
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
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
	bs, err := application.NewDownloader().Download(url)
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
				continue
			}

			// Close response body immediately after checking status
			statusCode := resp.StatusCode
			if err = resp.Body.Close(); err != nil {
				return fmt.Errorf("error waiting for signature-aggregator readiness %w", err)
			}

			// Check for various status codes
			switch statusCode {
			case http.StatusBadRequest:
				// A 400 means the service is up but received a malformed request
				return nil
			case http.StatusOK:
				// Service is up and responding correctly
				return nil
			case http.StatusServiceUnavailable:
				// Service is not ready yet
				continue
			default:
				// Log unexpected status codes but continue trying
				continue
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

func CreateSignatureAggregatorConfig(networkEndpoint string, apiPort, metricsPort int) *SignatureAggregatorConfig {
	config := &SignatureAggregatorConfig{
		LogLevel:             "debug",
		PChainAPI:            APIConfig{BaseURL: networkEndpoint},
		InfoAPI:              APIConfig{BaseURL: networkEndpoint},
		SignatureCacheSize:   1048576,
		AllowPrivateIPs:      true,
		TrackedSubnetIDs:     []string{},
		ManuallyTrackedPeers: make([]PeerConfig, 0),
		APIPort:              apiPort,
		MetricsPort:          metricsPort,
	}

	return config
}

// WriteSignatureAggregatorConfig writes the signature aggregator configuration to a file.
func WriteSignatureAggregatorConfig(config *SignatureAggregatorConfig, configPath string) error {
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
	if err = conn.Close(); err != nil {
		// Log the error but still return false since port is in use
		ux.Logger.RedXToUser("failed to close connection while checking port %d: %s", port, err)
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

func CreateSignatureAggregatorInstance(app *application.Avalanche, network models.Network, aggregatorLogger logging.Logger, version string) error {
	// Create config file for signature aggregator
	var apiPort, metricsPort int
	var err error
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	// Check if run file exists and read ports from it
	if _, err := os.Stat(runFilePath); err == nil {
		// File exists, get process details
		runFile, err := GetCurrentSignatureAggregatorProcessDetails(app, network)
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

	config := CreateSignatureAggregatorConfig(network.Endpoint, apiPort, metricsPort)
	configPath := filepath.Join(app.GetSignatureAggregatorRunDir(network.Kind), "config.json")
	if err := WriteSignatureAggregatorConfig(config, configPath); err != nil {
		return fmt.Errorf("failed to write signature aggregator config: %w", err)
	}
	logPath := filepath.Join(app.GetSignatureAggregatorRunDir(network.Kind), "signature-aggregator.log")
	signatureAggregatorEndpoint := fmt.Sprintf("http://localhost:%d/aggregate-signatures", apiPort)
	pid, err := StartSignatureAggregator(app, network, configPath, logPath, aggregatorLogger, version, signatureAggregatorEndpoint)
	if err != nil {
		return fmt.Errorf("failed to start signature aggregator: %w", err)
	}

	return saveSignatureAggregatorFile(runFilePath, pid, apiPort, metricsPort, version)
}

func GetSignatureAggregatorEndpoint(app *application.Avalanche, network models.Network) (string, error) {
	runFile, err := GetCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return "", fmt.Errorf("failed to get process details: %w", err)
	}
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
	if err := os.MkdirAll(filepath.Dir(runFilePath), constants.DefaultPerms755); err != nil {
		return err
	}
	if err := os.WriteFile(runFilePath, bs, constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("could not write signature aggregator run file to %s: %w", runFilePath, err)
	}
	return nil
}

// GetCurrentSignatureAggregatorProcessDetails reads the run file and returns the current process details.
// It returns the run file information including PID, ports, and version.
func GetCurrentSignatureAggregatorProcessDetails(app *application.Avalanche, network models.Network) (*signatureAggregatorRunFile, error) {
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

	runFile, err := GetCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		return fmt.Errorf("failed to get process details: %w", err)
	}

	// Kill existing process if running
	if runFile.Pid > 0 {
		process, err := os.FindProcess(runFile.Pid)
		if err == nil {
			if err := process.Kill(); err != nil {
				ux.Logger.RedXToUser("Failed to kill process %d: %v\n", runFile.Pid, err)
			}
		}
	}

	// Wait a bit for the process to be killed
	time.Sleep(2 * time.Second)
	return nil
}

// SignatureAggregatorCleanup cleans up the signature aggregator process and files.
// It removes the log file and run file, and stops the running process if any.
func SignatureAggregatorCleanup(
	app *application.Avalanche,
	network models.Network,
) error {
	runFilePath := app.GetLocalSignatureAggregatorRunPath(network.Kind)
	if _, err := os.Stat(runFilePath); os.IsNotExist(err) {
		return nil
	}
	runFile, err := GetCurrentSignatureAggregatorProcessDetails(app, network)
	if err != nil {
		// If we can't get process details, just continue with cleanup
		ux.Logger.RedXToUser("unable to get signature aggregator process details: %s", err)
	} else {
		// Force kill the process
		if err := syscall.Kill(runFile.Pid, syscall.SIGKILL); err != nil {
			// Just log the error and continue with cleanup
			ux.Logger.RedXToUser("unable to kill signature aggregator process with pid %d: %s", runFile.Pid, err)
		}
	}

	// Remove the entire signature aggregator directory regardless of process state
	signatureAggregatorDir := app.GetSignatureAggregatorRunDir(network.Kind)
	if err := os.RemoveAll(signatureAggregatorDir); err != nil {
		return fmt.Errorf("failed removing signature aggregator directory %s: %w", signatureAggregatorDir, err)
	}
	return nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

const (
	maxResponseSize      = 102400          // 100KB should be enough to read the avalanchego response
	sshConnectionTimeout = 3 * time.Second // usually takes less than 2
	sshConnectionRetries = 5
)

type Host struct {
	NodeID            string
	IP                string
	SSHUser           string
	SSHPrivateKeyPath string
	SSHCommonArgs     string
	Connection        *goph.Client
	APINode           bool
}

func NewHostConnection(h *Host, port uint) (*goph.Client, error) {
	if port == 0 {
		port = constants.SSHTCPPort
	}
	var (
		auth goph.Auth
		err  error
	)

	if h.SSHPrivateKeyPath == "" {
		auth, err = goph.UseAgent()
	} else {
		auth, err = goph.Key(h.SSHPrivateKeyPath, "")
	}
	if err != nil {
		return nil, err
	}
	cl, err := goph.NewConn(&goph.Config{
		User:    h.SSHUser,
		Addr:    h.IP,
		Port:    port,
		Auth:    auth,
		Timeout: sshConnectionTimeout,
		// #nosec G106
		Callback: ssh.InsecureIgnoreHostKey(), // we don't verify host key ( similar to ansible)
	})
	if err != nil {
		return nil, err
	}
	return cl, nil
}

// GetCloudID returns the node ID of the host.
func (h *Host) GetCloudID() string {
	_, cloudID, _ := HostAnsibleIDToCloudID(h.NodeID)
	return cloudID
}

// Connect starts a new SSH connection with the provided private key.
func (h *Host) Connect(port uint) error {
	if port == 0 {
		port = constants.SSHTCPPort
	}
	if h.Connection != nil {
		return nil
	}
	var err error
	for i := 0; h.Connection == nil && i < sshConnectionRetries; i++ {
		h.Connection, err = NewHostConnection(h, port)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to host %s: %w", h.IP, err)
	}
	return nil
}

func (h *Host) Connected() bool {
	return h.Connection != nil
}

func (h *Host) Disconnect() error {
	if h.Connection == nil {
		return nil
	}
	err := h.Connection.Close()
	return err
}

// Upload uploads a local file to a remote file on the host.
func (h *Host) Upload(localFile string, remoteFile string, timeout time.Duration) error {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}
	_, err := utils.TimedFunction(
		func() (any, error) {
			return nil, h.Connection.Upload(localFile, remoteFile)
		},
		"upload",
		timeout,
	)
	if err != nil {
		err = fmt.Errorf("%w for host %s", err, h.IP)
	}
	return err
}

// UploadBytes uploads a byte array to a remote file on the host.
func (h *Host) UploadBytes(data []byte, remoteFile string, timeout time.Duration) error {
	tmpFile, err := os.CreateTemp("", "HostUploadBytes-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return h.Upload(tmpFile.Name(), remoteFile, timeout)
}

// Download downloads a file from the remote server to the local machine.
func (h *Host) Download(remoteFile string, localFile string, timeout time.Duration) error {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(localFile), os.ModePerm); err != nil {
		return err
	}
	_, err := utils.TimedFunction(
		func() (any, error) {
			return nil, h.Connection.Download(remoteFile, localFile)
		},
		"download",
		timeout,
	)
	if err != nil {
		err = fmt.Errorf("%w for host %s", err, h.IP)
	}
	return err
}

// ReadFileBytes downloads a file from the remote server to a byte array
func (h *Host) ReadFileBytes(remoteFile string, timeout time.Duration) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "HostDownloadBytes-*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if err := h.Download(remoteFile, tmpFile.Name(), timeout); err != nil {
		return nil, err
	}
	return os.ReadFile(tmpFile.Name())
}

// ExpandHome expands the ~ symbol to the home directory.
func (h *Host) ExpandHome(path string) string {
	userHome := filepath.Join("/home", h.SSHUser)
	if path == "" {
		return userHome
	}
	if len(path) > 0 && path[0] == '~' {
		path = filepath.Join(userHome, path[1:])
	}
	return path
}

// MkdirAll creates a folder on the remote server.
func (h *Host) MkdirAll(remoteDir string, timeout time.Duration) error {
	remoteDir = h.ExpandHome(remoteDir)
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}
	_, err := utils.TimedFunction(
		func() (any, error) {
			return nil, h.UntimedMkdirAll(remoteDir)
		},
		"mkdir",
		timeout,
	)
	if err != nil {
		err = fmt.Errorf("%w for host %s", err, h.IP)
	}
	return err
}

// UntimedMkdirAll creates a folder on the remote server.
// Does not support timeouts on the operation.
func (h *Host) UntimedMkdirAll(remoteDir string) error {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}
	sftp, err := h.Connection.NewSftp()
	if err != nil {
		return err
	}
	defer sftp.Close()
	return sftp.MkdirAll(remoteDir)
}

// Command executes a shell command on a remote host.
func (h *Host) Command(script string, env []string, timeout time.Duration) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// ux.Logger.Info("DEBUG Command on host %s: %s", h.IP, script)
	cmd, err := h.Connection.CommandContext(ctx, "", script)
	if err != nil {
		return nil, err
	}
	if env != nil {
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	return output, err
}

// Forward forwards the TCP connection to a remote address.
func (h *Host) Forward(httpRequest string, timeout time.Duration) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return nil, err
		}
	}
	ret, err := utils.TimedFunctionWithRetry(
		func() ([]byte, error) {
			return h.UntimedForward(httpRequest)
		},
		"post over ssh",
		timeout,
		3,
		2*time.Second,
	)
	if err != nil {
		err = fmt.Errorf("%w for host %s", err, h.IP)
	}
	return ret, err
}

// UntimedForward forwards the TCP connection to a remote address.
// Does not support timeouts on the operation.
func (h *Host) UntimedForward(httpRequest string) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return nil, err
		}
	}
	avalancheGoEndpoint := strings.TrimPrefix(constants.LocalAPIEndpoint, "http://")
	avalancheGoAddr, err := net.ResolveTCPAddr("tcp", avalancheGoEndpoint)
	if err != nil {
		return nil, err
	}
	var proxy net.Conn
	if utils.IsE2E() {
		avalancheGoEndpoint = fmt.Sprintf("%s:%d", utils.E2EConvertIP(h.IP), constants.AvalancheGoAPIPort)
		proxy, err = net.Dial("tcp", avalancheGoEndpoint)
		if err != nil {
			return nil, fmt.Errorf("unable to port forward E2E to %s", avalancheGoEndpoint)
		}
	} else {
		proxy, err = h.Connection.DialTCP("tcp", nil, avalancheGoAddr)
		if err != nil {
			return nil, fmt.Errorf("unable to port forward to %s via %s", h.Connection.RemoteAddr(), "ssh")
		}
	}

	defer proxy.Close()
	// send request to server
	if _, err = proxy.Write([]byte(httpRequest)); err != nil {
		return nil, err
	}
	// Read and print the server's response
	response := make([]byte, maxResponseSize)
	responseLength, err := proxy.Read(response)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(bytes.NewReader(response[:responseLength]))
	parsedResponse, err := http.ReadResponse(reader, nil)
	if err != nil {
		return nil, err
	}
	buffer := new(bytes.Buffer)
	if _, err = buffer.ReadFrom(parsedResponse.Body); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// FileExists checks if a file exists on the remote server.
func (h *Host) FileExists(path string) (bool, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return false, err
		}
	}

	sftp, err := h.Connection.NewSftp()
	if err != nil {
		return false, nil
	}
	defer sftp.Close()
	_, err = sftp.Stat(path)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CreateTempFile creates a temporary file on the remote server.
func (h *Host) CreateTempFile() (string, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return "", err
		}
	}
	sftp, err := h.Connection.NewSftp()
	if err != nil {
		return "", err
	}
	defer sftp.Close()
	tmpFileName := filepath.Join("/tmp", utils.RandomString(10))
	_, err = sftp.Create(tmpFileName)
	if err != nil {
		return "", err
	}
	return tmpFileName, nil
}

// CreateTempDir creates a temporary directory on the remote server.
func (h *Host) CreateTempDir() (string, error) {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return "", err
		}
	}
	sftp, err := h.Connection.NewSftp()
	if err != nil {
		return "", err
	}
	defer sftp.Close()
	tmpDirName := filepath.Join("/tmp", utils.RandomString(10))
	err = sftp.Mkdir(tmpDirName)
	if err != nil {
		return "", err
	}
	return tmpDirName, nil
}

// Remove removes a file on the remote server.
func (h *Host) Remove(path string, recursive bool) error {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}
	sftp, err := h.Connection.NewSftp()
	if err != nil {
		return err
	}
	defer sftp.Close()
	if recursive {
		// return sftp.RemoveAll(path) is very slow
		_, err := h.Command(fmt.Sprintf("rm -rf %s", path), nil, constants.SSHLongRunningScriptTimeout)
		return err
	} else {
		return sftp.Remove(path)
	}
}

func (h *Host) GetAnsibleInventoryRecord() string {
	return strings.Join([]string{
		h.NodeID,
		fmt.Sprintf("ansible_host=%s", h.IP),
		fmt.Sprintf("ansible_user=%s", h.SSHUser),
		fmt.Sprintf("ansible_ssh_private_key_file=%s", h.SSHPrivateKeyPath),
		fmt.Sprintf("ansible_ssh_common_args='%s'", h.SSHCommonArgs),
	}, " ")
}

func HostCloudIDToAnsibleID(cloudService string, hostCloudID string) (string, error) {
	switch cloudService {
	case constants.GCPCloudService:
		return fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, hostCloudID), nil
	case constants.AWSCloudService:
		return fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, hostCloudID), nil
	case constants.E2EDocker:
		return fmt.Sprintf("%s_%s", constants.E2EDocker, hostCloudID), nil
	}
	return "", fmt.Errorf("unknown cloud service %s", cloudService)
}

// HostAnsibleIDToCloudID converts a host Ansible ID to a cloud ID.
func HostAnsibleIDToCloudID(hostAnsibleID string) (string, string, error) {
	var cloudService, cloudIDPrefix string
	switch {
	case strings.HasPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix):
		cloudService = constants.AWSCloudService
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix+"_")
	case strings.HasPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix):
		cloudService = constants.GCPCloudService
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix+"_")
	case strings.HasPrefix(hostAnsibleID, constants.E2EDocker):
		cloudService = constants.E2EDocker
		cloudIDPrefix = strings.TrimPrefix(hostAnsibleID, constants.E2EDocker+"_")
	default:
		return "", "", fmt.Errorf("unknown cloud service prefix in %s", hostAnsibleID)
	}
	return cloudService, cloudIDPrefix, nil
}

// WaitForPort waits for the SSH port to become available on the host.
func (h *Host) WaitForPort(port uint, timeout time.Duration) error {
	if port == 0 {
		port = constants.SSHTCPPort
	}
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: SSH port %d on host %s is not available after %vs", port, h.IP, timeout.Seconds())
		}
		if _, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", h.IP, port), time.Second); err == nil {
			return nil
		}
		time.Sleep(constants.SSHSleepBetweenChecks)
	}
}

// WaitForSSHShell waits for the SSH shell to be available on the host within the specified timeout.
func (h *Host) WaitForSSHShell(timeout time.Duration) error {
	if h.IP == "" {
		return fmt.Errorf("host IP is empty")
	}
	start := time.Now()
	if err := h.WaitForPort(constants.SSHTCPPort, timeout); err != nil {
		return err
	}

	deadline := start.Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: SSH shell on host %s is not available after %ds", h.IP, int(timeout.Seconds()))
		}
		if err := h.Connect(0); err != nil {
			time.Sleep(constants.SSHSleepBetweenChecks)
			continue
		}
		if h.Connected() {
			output, err := h.Command("echo", nil, timeout)
			if err == nil || len(output) > 0 {
				return nil
			}
		}
		time.Sleep(constants.SSHSleepBetweenChecks)
	}
}

// StreamSSHCommand streams the execution of an SSH command on the host.
func (h *Host) StreamSSHCommand(command string, env []string, timeout time.Duration) error {
	if !h.Connected() {
		if err := h.Connect(0); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	session, err := h.Connection.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}
	for _, item := range env {
		envPair := strings.SplitN(item, "=", 2)
		if len(envPair) != 2 {
			return fmt.Errorf("invalid env variable %s", item)
		}
		if err := session.Setenv(envPair[0], envPair[1]); err != nil {
			return err
		}
	}
	// Use a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := consumeOutput(ctx, stdout); err != nil {
			fmt.Printf("Error reading stdout: %v\n", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := consumeOutput(ctx, stderr); err != nil {
			fmt.Printf("Error reading stderr: %v\n", err)
		}
	}()

	if err := session.Run(command); err != nil {
		return fmt.Errorf("failed to run command %s: %w", command, err)
	}
	wg.Wait()
	return nil
}

func consumeOutput(ctx context.Context, output io.Reader) error {
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		// Check if the context is done
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return scanner.Err()
}

// HasSystemDAvaliable checks if systemd is available on a remote host.
func (h *Host) IsSystemD() bool {
	// check for the folder
	if _, err := h.FileExists("/run/systemd/system"); err != nil {
		return false
	}
	tmpFile, err := os.CreateTemp("", "avalanchecli-proc-systemd-*.txt")
	if err != nil {
		return false
	}
	defer os.Remove(tmpFile.Name())
	// check for the service
	if err := h.Download("/proc/1/comm", tmpFile.Name(), constants.SSHFileOpsTimeout); err != nil {
		return false
	}
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "systemd"
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
}

func NewHostConnection(h *Host) (*goph.Client, error) {
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
		Port:    constants.SSHTCPPort,
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
func (h *Host) Connect() error {
	if h.Connection != nil {
		return nil
	}
	var err error
	for i := 0; h.Connection == nil && i < sshConnectionRetries; i++ {
		h.Connection, err = NewHostConnection(h)
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
		if err := h.Connect(); err != nil {
			return err
		}
	}
	_, err := utils.TimedFunction(
		func() (interface{}, error) {
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

// Download downloads a file from the remote server to the local machine.
func (h *Host) Download(remoteFile string, localFile string, timeout time.Duration) error {
	if !h.Connected() {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(localFile), os.ModePerm); err != nil {
		return err
	}
	_, err := utils.TimedFunction(
		func() (interface{}, error) {
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

// MkdirAll creates a folder on the remote server.
func (h *Host) MkdirAll(remoteDir string, timeout time.Duration) error {
	if !h.Connected() {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	_, err := utils.TimedFunction(
		func() (interface{}, error) {
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
		if err := h.Connect(); err != nil {
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
		if err := h.Connect(); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, err := h.Connection.CommandContext(ctx, constants.SSHShell, script)
	if err != nil {
		return nil, err
	}
	if env != nil {
		cmd.Env = env
	}
	return cmd.CombinedOutput()
}

// Forward forwards the TCP connection to a remote address.
func (h *Host) Forward(httpRequest string, timeout time.Duration) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(); err != nil {
			return nil, err
		}
	}
	retI, err := utils.TimedFunction(
		func() (interface{}, error) {
			return h.UntimedForward(httpRequest)
		},
		"post over ssh",
		timeout,
	)
	if err != nil {
		err = fmt.Errorf("%w for host %s", err, h.IP)
	}
	ret := []byte(nil)
	if retI != nil {
		ret = retI.([]byte)
	}
	return ret, err
}

// UntimedForward forwards the TCP connection to a remote address.
// Does not support timeouts on the operation.
func (h *Host) UntimedForward(httpRequest string) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(); err != nil {
			return nil, err
		}
	}
	avalancheGoEndpoint := strings.TrimPrefix(constants.LocalAPIEndpoint, "http://")
	avalancheGoAddr, err := net.ResolveTCPAddr("tcp", avalancheGoEndpoint)
	if err != nil {
		return nil, err
	}
	proxy, err := h.Connection.DialTCP("tcp", nil, avalancheGoAddr)
	if err != nil {
		return nil, fmt.Errorf("unable to port forward to %s via %s", h.Connection.RemoteAddr(), "ssh")
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
	}
	return "", fmt.Errorf("unknown cloud service %s", cloudService)
}

func HostAnsibleIDToCloudID(hostAnsibleID string) (string, string, error) {
	if strings.HasPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix) {
		return constants.AWSCloudService, strings.TrimPrefix(hostAnsibleID, constants.AWSNodeAnsiblePrefix+"_"), nil
	} else if strings.HasPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix) {
		return constants.GCPCloudService, strings.TrimPrefix(hostAnsibleID, constants.GCPNodeAnsiblePrefix+"_"), nil
	}
	return "", "", fmt.Errorf("unknown cloud service prefix in %s", hostAnsibleID)
}

// WaitForSSHPort waits for the SSH port to become available on the host.
func (h *Host) WaitForSSHPort(timeout time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: SSH port %d on host %s is not available after %vs", constants.SSHTCPPort, h.IP, timeout.Seconds())
		}
		if _, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", h.IP, constants.SSHTCPPort), time.Second); err == nil {
			return nil
		}
		time.Sleep(constants.SSHSleepBetweenChecks)
	}
}

// WaitForSSHShell waits for the SSH shell to be available on the host within the specified timeout.
func (h *Host) WaitForSSHShell(timeout time.Duration) error {
	start := time.Now()
	if err := h.WaitForSSHPort(timeout); err != nil {
		return err
	}
	deadline := start.Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: SSH shell on host %s is not available after %ds", h.IP, int(timeout.Seconds()))
		}
		if err := h.Connect(); err != nil {
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

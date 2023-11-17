// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

const (
	maxResponseSize = 102400 // 100KB should be enough to read the avalanchego response
)

type Host struct {
	NodeID            string
	IP                string
	SSHUser           string
	SSHPrivateKeyPath string
	SSHCommonArgs     string
	Connection        *HostConnection
}

type HostConnection struct {
	Client    *goph.Client
	Ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewHostConnection(h Host, timeout time.Duration) *HostConnection {
	if h.Connection != nil { // reuse connection if it exists
		return h.Connection
	}
	p := new(HostConnection)
	if timeout == 0 {
		timeout = constants.SSHScriptTimeout
	}
	p.Ctx, p.ctxCancel = context.WithTimeout(context.Background(), timeout)
	auth, err := goph.Key(h.SSHPrivateKeyPath, "")
	if err != nil {
		return nil
	}
	cl, err := goph.NewConn(&goph.Config{
		User:    h.SSHUser,
		Addr:    h.IP,
		Port:    22,
		Auth:    auth,
		Timeout: timeout,
		// #nosec G106
		Callback: ssh.InsecureIgnoreHostKey(), // we don't verify host key ( similar to ansible)
	})
	if err != nil {
		return nil
	} else {
		p.Client = cl
	}
	return p
}

// GetCloudID returns the node ID of the host.
func (h *Host) GetCloudID() string {
	_, cloudID, _ := HostAnsibleIDToCloudID(h.NodeID)
	return cloudID
}

// Connect starts a new SSH connection with the provided private key.
func (h *Host) Connect(timeout time.Duration) error {
	h.Connection = NewHostConnection(*h, timeout)
	if !h.Connected() {
		return fmt.Errorf("failed to connect to host %s", h.IP)
	}
	return nil
}

func (h *Host) Connected() bool {
	return h.Connection != nil
}

func (h *Host) Disconnect() error {
	if !h.Connected() {
		return nil
	}
	return h.Connection.Client.Close()
}

// Upload uploads a local file to a remote file on the host.
func (h *Host) Upload(localFile string, remoteFile string) error {
	if !h.Connected() {
		if err := h.Connect(constants.SSHFileOpsTimeout); err != nil {
			return err
		}
	}
	return h.Connection.Client.Upload(localFile, remoteFile)
}

// Download downloads a file from the remote server to the local machine.
func (h *Host) Download(remoteFile string, localFile string) error {
	if !h.Connected() {
		if err := h.Connect(constants.SSHScriptTimeout); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(localFile), os.ModePerm); err != nil {
		return err
	}
	return h.Connection.Client.Download(remoteFile, localFile)
}

// MkdirAll creates a folder on the remote server.
func (h *Host) MkdirAll(remoteDir string) error {
	if !h.Connected() {
		if err := h.Connect(constants.SSHScriptTimeout); err != nil {
			return err
		}
	}
	sftp, err := h.Connection.Client.NewSftp()
	if err != nil {
		return err
	}
	defer sftp.Close()
	return sftp.MkdirAll(remoteDir)
}

// Command executes a shell command on a remote host.
func (h *Host) Command(script string, env []string, ctx context.Context) ([]byte, error) {
	if !h.Connected() {
		if err := h.Connect(constants.SSHScriptTimeout); err != nil {
			return nil, err
		}
	}
	if h.Connected() {
		cmd, err := h.Connection.Client.CommandContext(ctx, constants.SSHShell, script)
		if err != nil {
			return nil, err
		}
		if env != nil {
			cmd.Env = env
		}
		return cmd.CombinedOutput()
	} else {
		return nil, fmt.Errorf("failed to connect to host %s", h.IP)
	}
}

// Forward forwards the TCP connection to a remote address.
func (h *Host) Forward(httpRequest string) ([]byte, []byte, error) {
	if !h.Connected() {
		if err := h.Connect(constants.SSHPOSTTimeout); err != nil {
			return nil, nil, err
		}
	}
	avalancheGoEndpoint := strings.TrimPrefix(constants.LocalAPIEndpoint, "http://")
	avalancheGoAddr, err := net.ResolveTCPAddr("tcp", avalancheGoEndpoint)
	if err != nil {
		return nil, nil, err
	}
	proxy, err := h.Connection.Client.DialTCP("tcp", nil, avalancheGoAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to port forward to %s via %s", h.Connection.Client.Conn.RemoteAddr(), "ssh")
	}
	defer proxy.Close()
	// send request to server
	_, err = proxy.Write([]byte(httpRequest))
	if err != nil {
		return nil, nil, err
	}
	// Read and print the server's response
	response := make([]byte, maxResponseSize)
	responseLength, err := proxy.Read(response)
	if err != nil {
		return nil, nil, err
	}
	header, body := SplitHTTPResponse(response[:responseLength])
	return header, body, nil
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
	if err := h.WaitForSSHPort(timeout); err != nil {
		return err
	}
	start := time.Now()
	deadline := start.Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: SSH shell on host %s is not available after %ds", h.IP, int(timeout.Seconds()))
		}
		if err := h.Connect(timeout); err != nil {
			time.Sleep(constants.SSHSleepBetweenChecks)
			continue
		}
		if h.Connected() {
			output, err := h.Command("echo", nil, context.Background())
			if err == nil || len(output) > 0 {
				return nil
			}
		}
		time.Sleep(constants.SSHSleepBetweenChecks)
	}
}

// splitHTTPResponse splits an HTTP response into headers and body.
func SplitHTTPResponse(response []byte) ([]byte, []byte) {
	// Find the position of the double line break separating the headers and the body
	doubleLineBreak := []byte{'\r', '\n', '\r', '\n'}
	index := bytes.Index(response, doubleLineBreak)
	if index == -1 {
		return nil, response
	}
	// Split the response into headers and body
	headers := response[:index]
	body := response[index+len(doubleLineBreak):]
	return headers, body
}

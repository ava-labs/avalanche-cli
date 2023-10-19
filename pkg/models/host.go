// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

type Host struct {
	NodeID            string
	IP                string
	SSHUser           string
	SSHPrivateKeyPath string
	SSHCommonArgs     string
}

const (
	shell     = "/bin/bash"
	localhost = "127.0.0.1"
)

// GetNodeID returns the node ID of the host.
//
// It checks if the node ID has a prefix of constants.AnsibleAWSNodePrefix
// and removes the prefix if present. Otherwise, it joins the first two parts
// of the node ID split by "_" and returns the result.
//
// Returns:
//   - string: The node ID of the host.
func (h Host) GetInstanceID() string {
	if strings.HasPrefix(h.NodeID, constants.AnsibleAWSNodePrefix) {
		return strings.TrimPrefix(h.NodeID, constants.AnsibleAWSNodePrefix)
	}
	// default behaviour - TODO refactor for other clouds
	return strings.Join(strings.Split(h.NodeID, "_")[:2], "_")
}

// Connect starts a new SSH connection with the provided private key.
//
// It returns a pointer to a goph.Client and an error.
func (h Host) Connect(timeout time.Duration) (*goph.Client, error) {
	if timeout == 0 {
		timeout = constants.SSHScriptTimeout
	}
	// Start new ssh connection with private key.
	auth, err := goph.Key(h.SSHPrivateKeyPath, "")
	if err != nil {
		return nil, err
	}
	client, err := goph.NewConn(&goph.Config{
		User:     h.SSHUser,
		Addr:     h.IP,
		Port:     22,
		Auth:     auth,
		Timeout:  timeout,
		Callback: ssh.InsecureIgnoreHostKey(), // #nosec G106
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Upload uploads a local file to a remote file on the host.
//
// localFile: the path of the local file to be uploaded.
// remoteFile: the path of the remote file to be created or overwritten.
// error: an error if there was a problem during the upload process.
func (h Host) Upload(localFile string, remoteFile string) error {
	client, err := h.Connect(constants.SSHFileOpsTimeout)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Upload(localFile, remoteFile)
}

// Download downloads a file from the remote server to the local machine.
//
// remoteFile: the path to the file on the remote server.
// localFile: the path to the file on the local machine.
// error: returns an error if there was a problem downloading the file.
func (h Host) Download(remoteFile string, localFile string) error {
	client, err := h.Connect(constants.SSHFileOpsTimeout)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Download(remoteFile, localFile)
}

// Command executes a shell command on a remote host.
//
// It takes a script string, an environment []string, and a context.Context as parameters.
// It returns a *goph.Cmd and an error.
func (h Host) Command(script string, env []string, ctx context.Context) ([]byte, error) {
	client, err := h.Connect(constants.SSHScriptTimeout)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	cmd, err := client.CommandContext(ctx, shell, script)
	if err != nil {
		return nil, err
	}
	if env != nil {
		cmd.Env = env
	}
	return cmd.CombinedOutput()
}

// Forward forwards the TCP connection to a remote address.
//
// It returns an error if there was an issue connecting to the remote address or if there was an error in the port forwarding process.
func (h Host) Forward(httpRequest string) ([]byte, []byte, error) {
	client, err := h.Connect(constants.SSHPOSTTimeout)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()
	avalancheGoEndpoint := strings.TrimPrefix(constants.LocalAPIEndpoint, "http://")
	avalancheGoAddr, err := net.ResolveTCPAddr("tcp", avalancheGoEndpoint)
	if err != nil {
		return nil, nil, err
	}
	proxy, err := client.DialTCP("tcp", nil, avalancheGoAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to port forward to %s via %s", client.Conn.RemoteAddr(), "ssh")
	}
	defer proxy.Close()
	// send request to server
	_, err = proxy.Write([]byte(httpRequest))
	if err != nil {
		return nil, nil, err
	}
	// Read and print the server's response
	response := make([]byte, 10240)
	responseLength, err := proxy.Read(response)
	if err != nil {
		return nil, nil, err
	}
	header, body := SplitHTTPResponse(response[0 : responseLength-1])
	return header, body, nil
}

// ConvertToNodeID converts a node name to a node ID.
//
// It takes a nodeName string as a parameter and returns a string representing the node ID.
func (h Host) ConvertToInstanceID(nodeID string) string {
	h = Host{
		NodeID:            nodeID,
		SSHUser:           "ubuntu",
		SSHPrivateKeyPath: "",
		SSHCommonArgs:     "",
	}
	return h.GetInstanceID()
}

// GetAnsibleParams returns the string representation of the Ansible parameters for the Host.
//
// No parameters.
// Returns a string.
func (h Host) GetAnsibleParams() string {
	return strings.Join([]string{
		fmt.Sprintf("ansible_host=%s", h.IP),
		fmt.Sprintf("ansible_user=%s", h.SSHUser),
		fmt.Sprintf("ansible_ssh_private_key_file=%s", h.SSHPrivateKeyPath),
		fmt.Sprintf("ansible_ssh_common_args='%s'", h.SSHCommonArgs),
	}, " ")
}

// splitHTTPResponse splits an HTTP response into headers and body.
//
// It takes a byte slice `response` as a parameter, which represents the HTTP response.
// The function returns two byte slices - `headers` and `body` - representing the headers and body of the response, respectively.
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

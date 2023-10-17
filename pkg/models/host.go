// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"

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
	client            *goph.Client
	TCPProxy          *bytes.Buffer
}

const (
	shell     = "/bin/bash"
	localhost = "127.0.0.1"
)

func (h Host) CloudNodeID() string {
	if strings.HasPrefix(h.NodeID, constants.AnsibleAWSNodePrefix) {
		return strings.TrimPrefix(h.NodeID, constants.AnsibleAWSNodePrefix)
	}
	//default behaviour - TODO refactor for other clouds
	return strings.Join(strings.Split(h.NodeID, "_")[:2], "_")
}

func (h Host) Connect() error {
	// Start new ssh connection with private key.
	auth, err := goph.Key(h.SSHPrivateKeyPath, "")
	if err != nil {
		return err
	}
	//client, err := goph.NewUnknown(h.SSHUser, h.IP, auth)
	client, err := goph.NewConn(&goph.Config{
		User:     h.SSHUser,
		Addr:     h.IP,
		Port:     22,
		Auth:     auth,
		Timeout:  constants.DefaultSSHTimeout,
		Callback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return err
	}
	h.client = client
	return nil
}

func (h Host) Upload(localFile string, remoteFile string) error {
	if h.client == nil {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	return h.client.Upload(localFile, remoteFile)
}

func (h Host) Download(remoteFile string, localFile string) error {
	if h.client == nil {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	return h.client.Download(remoteFile, localFile)
}

func (h Host) Close() error {
	if h.client == nil {
		return nil
	}

	return h.client.Close()
}

func (h Host) Command(script string, env []string, ctx context.Context) (*goph.Cmd, error) {
	fmt.Println("about to connect via ssh")
	if h.client == nil {
		if err := h.Connect(); err != nil {
			return nil, err
		}
	}
	fmt.Println("connected")
	cmd, err := h.client.CommandContext(ctx, shell, script)
	if err != nil {
		return nil, err
	}
	fmt.Println("context")
	cmd.Env = env
	fmt.Println("env")
	fmt.Println(cmd)
	return cmd, nil
}

func (h Host) Forward() error {
	if h.client == nil {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	remoteAddr, err := net.ResolveTCPAddr("tcp", constants.LocalAPIEndpoint)
	if err != nil {
		return err
	}
	proxy, err := h.client.DialTCP("tcp", nil, remoteAddr)
	if err != nil {
		return fmt.Errorf("unable to port forward to %s via %s", constants.LocalAPIEndpoint, "ssh")
	}

	errorChan := make(chan error)

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		_, err = io.Copy(h.TCPProxy, proxy)
		if err != nil {
			errorChan <- err
		}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		_, err = io.Copy(proxy, h.TCPProxy)
		if err != nil {
			errorChan <- err
		}
	}()
	return nil
}

func (h Host) GetNodeID() string {
	return strings.TrimPrefix(h.NodeID, constants.AnsibleAWSNodePrefix)
}

func (h Host) ConvertToNodeID(nodeName string) string {
	h = Host{
		NodeID:            nodeName,
		SSHUser:           "ubuntu",
		SSHPrivateKeyPath: "",
		SSHCommonArgs:     "",
	}
	return h.GetNodeID()
}

func (h Host) GetAnsibleParams() string {
	return strings.Join([]string{
		fmt.Sprintf("ansible_host=%s", h.IP),
		fmt.Sprintf("ansible_user=%s", h.SSHUser),
		fmt.Sprintf("ansible_ssh_private_key_file=%s", h.SSHPrivateKeyPath),
		fmt.Sprintf("ansible_ssh_common_args='%s'", h.SSHCommonArgs),
	}, " ")
}

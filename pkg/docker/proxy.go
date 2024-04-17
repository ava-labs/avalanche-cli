// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func StartSwarmProxy(hosts []models.Host, localSocketPath string, stopCh chan struct{}) {
	_ = os.Remove(localSocketPath)

	l, err := net.Listen("unix", localSocketPath)
	if err != nil {
		ux.Logger.Error("Failed to listen on local socket: %v", err)
	}
	defer l.Close()

	ux.Logger.Info("Local Swarm proxy listening on", localSocketPath)

	// Accept connections and proxy them to remote socket
	for {
		conn, err := l.Accept()
		if err != nil {
			ux.Logger.Error("Failed to accept connection: %v", err)
			continue
		}
		swarmHost, err := DiscoverSwarmHost(hosts)
		if err != nil {
			ux.Logger.Error("Failed to discover Swarm host: %v", err)
			conn.Close()
			continue
		}
		go handleConnection(conn, swarmHost, constants.RemoteDockeSocketPath)
		select {
		case <-stopCh:
			conn.Close()
			return
		default:
			continue
		}
	}
}

func DiscoverSwarmHost(hosts []models.Host) (models.Host, error) {
	if len(hosts) == 0 {
		return models.Host{}, fmt.Errorf("no hosts provided")
	} else {
		for _, host := range hosts {
			if isDocker, err := host.FileExists(constants.RemoteDockeSocketPath); isDocker && err == nil {
				return host, nil
			}
		}
	}
	return models.Host{}, fmt.Errorf("no Docker available")
}

func handleConnection(conn net.Conn, host models.Host, remoteSocketPath string) {
	defer conn.Close()

	if !host.Connected() {
		if err := host.Connect(0); err != nil {
			ux.Logger.Error("Failed to connect to host: %v", err)
			return
		}
	}

	remoteConn, err := host.Connection.Dial("unix", remoteSocketPath)
	if err != nil {
		ux.Logger.Error("Failed to connect to remote socket: %v", err)
		return
	}
	defer remoteConn.Close()

	// Setup bidirectional copying
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(remoteConn, conn)
		if err != nil {
			ux.Logger.Error("Error copying from local to remote: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, remoteConn)
		if err != nil {
			ux.Logger.Error("Error copying from remote to local: %v", err)
		}
	}()

	// Wait for both copying processes to finish
	wg.Wait()
}

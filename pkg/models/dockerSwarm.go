// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type Swarm struct {
	SocketPath string
	stopCh     chan struct{}
	hosts      []Host
}

func NewSwarm(socketPath string, hosts []Host) (*Swarm, error) {
	stopCh := make(chan struct{})

	go docker.StartSwarmProxy(hosts, socketPath, stopCh)

	return &Swarm{
		SocketPath: socketPath,
		stopCh:     stopCh,
		hosts:      hosts,
	}, nil
}

func (s *Swarm) StopProxy() {
	s.stopCh <- struct{}{}
	close(s.stopCh)
}

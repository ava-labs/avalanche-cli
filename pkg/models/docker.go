// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

/*
type DockerProxy struct {
	SocketPath string
	stopCh     chan struct{}
	client     *client.Client
	ctx        context.Context
}

func NewDockerProxy(socketPath string, sdkHost.Host) (*DockerProxy, error) {
	stopCh := make(chan struct{})

	go StartDockerProxy(host, socketPath, stopCh)

	return &DockerProxy{
		SocketPath: socketPath,
		stopCh:     stopCh,
		ctx:        context.Background(),
	}, nil
}

func (s *DockerProxy) Stop() {
	s.stopCh <- struct{}{}
	close(s.stopCh)
}

func StartDockerProxy(dockersdkHost.Host, localSocketPath string, stopCh chan struct{}) {
	_ = os.Remove(localSocketPath)

	l, err := net.Listen("unix", localSocketPath)
	if err != nil {
		ux.Logger.Error("Failed to listen on local socket: %v", err)
	}
	defer l.Close()

	ux.Logger.Info("Local docker proxy listening on %s", localSocketPath)

	// Accept connections and proxy them to remote socket
	for {
		conn, err := l.Accept()
		if err != nil {
			ux.Logger.Error("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn, dockerHost, constants.RemoteDockeSocketPath)
		select {
		case <-stopCh:
			_ = conn.Close()
			return
		default:
			continue
		}
	}
}

func handleConnection(conn net.Conn, sdkHost.Host, remoteSocketPath string) {
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

	wg.Wait()
}

// ConnectViaProxy connects to the Docker daemon via the proxy
func (s *DockerProxy) Connect() error {
	var err error
	s.client, err = client.NewClientWithOpts(client.WithHost(fmt.Sprintf("unix://%s", s.SocketPath)))
	if err != nil {
		return err
	}
	return nil
}
*/

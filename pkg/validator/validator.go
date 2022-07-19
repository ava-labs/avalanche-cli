package validator

import (
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/kardianos/service"
)

type AvalanchegoValidator struct {
	Network  models.Network
	buildDir string
}

func (a *AvalanchegoValidator) Start(svc service.Service) error {
	svc.Start()
	return nil
}

func (a *AvalanchegoValidator) Stop(svc service.Service) error {
	return nil
}

func StartLocalNodeAsService(network models.Network, buildDir string) error {
	avago := &AvalanchegoValidator{
		Network:  network,
		buildDir: buildDir,
	}
	bin := filepath.Join(buildDir, "avalanchego")
	args := []string{"--network-id", strings.ToLower(network.String()), "--build-dir", buildDir}
	opts := map[string]interface{}{
		"UserService": true,
	}
	svcConfig := &service.Config{
		Name:        "avalanchego",
		DisplayName: "avalanchego",
		Description: "avalanchego node",
		Executable:  bin,
		Arguments:   args,
		Option:      opts,
	}
	svc, err := service.New(avago, svcConfig)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("starting a new avalanchego node for network %q as system service...", network.String())
	if err := service.Control(svc, "start"); err != nil {
		return err
	}
	ux.Logger.PrintToUser("node started, you can now use your system's service management to check on the node")
	return nil
}

func InstallAsAService(network models.Network) error {
	return nil
}

package validator

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/kardianos/service"
)

type AvalanchegoValidator struct {
	Network  models.Network
	buildDir string
}

func (a *AvalanchegoValidator) Start(svc service.Service) error {
	return nil
}

func (a *AvalanchegoValidator) Stop(svc service.Service) error {
	svc.Stop()
	return nil
}

func StopLocalNodeAsService(network models.Network, buildDir string, app *application.Avalanche) error {
	avago := &AvalanchegoValidator{
		Network:  network,
		buildDir: buildDir,
	}

	svcConfig, err := getServiceFile(network, buildDir, app)
	if err != nil {
		return err
	}

	svc, err := service.New(avago, svcConfig)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("stopping avalanchego %s node...", network.String())
	if err := service.Control(svc, "stop"); err != nil {
		return err
	}
	ux.Logger.PrintToUser("node stopped")
	return nil
}

func StartLocalNodeAsService(network models.Network, buildDir string, app *application.Avalanche) error {
	avago := &AvalanchegoValidator{
		Network:  network,
		buildDir: buildDir,
	}

	svcConfig, err := getServiceFile(network, buildDir, app)
	if err != nil {
		return err
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

func InstallAsAService(network models.Network, buildDir string, app *application.Avalanche) error {
	avago := &AvalanchegoValidator{
		Network:  network,
		buildDir: buildDir,
	}

	svcConfig, err := getServiceFile(network, buildDir, app)
	if err != nil {
		return err
	}

	svc, err := service.New(avago, svcConfig)
	if err != nil {
		return err
	}
	return svc.Install()
}

func GetStatus(app *application.Avalanche) (string, error) {
	// avago := &AvalanchegoValidator{}
	exists, err := serviceFileExists(app)
	if err != nil {
		return "service not installed", nil
	}
	// var svcConfig *service.Config
	if exists {
		/*
			svcConfig, err = loadServiceConfig(app)
			if err != nil {
				return "", err
			}
			svc, err := service.New(avago, svcConfig)
			if err != nil {
				return "", err
			}
		*/
		cmd := exec.Command("systemctl", "--user", "status", "avalanchego")
		output, _ := cmd.Output()
		return string(output), nil
	}
	return "service not installed", nil
}

func getServiceFile(network models.Network, bin string, app *application.Avalanche) (*service.Config, error) {
	exists, err := serviceFileExists(app)
	if err != nil {
		return nil, err
	}
	var svcConfig *service.Config
	if exists {
		svcConfig, err = loadServiceConfig(app)
		if err != nil {
			return nil, err
		}
	} else {
		buildDir := filepath.Dir(bin)
		args := []string{"--network-id", strings.ToLower(network.String()), "--build-dir", buildDir}
		opts := map[string]interface{}{
			"UserService": true,
		}
		svcConfig = &service.Config{
			Name:        "avalanchego",
			DisplayName: "avalanchego",
			Description: "avalanchego node",
			Executable:  bin,
			Arguments:   args,
			Option:      opts,
		}
		err = writeServiceConfig(app, svcConfig)
		if err != nil {
			return nil, err
		}
	}
	return svcConfig, nil
}

func serviceFileExists(app *application.Avalanche) (bool, error) {
	if _, err := os.Stat(app.GetServiceDir()); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(app.GetServiceDir(), constants.DefaultPerms755); err != nil {
				return false, err
			}
		} else {
			return false, err
		}
	}
	svcFile := filepath.Join(app.GetServiceDir(), constants.ServiceFile)
	if _, err := os.Stat(svcFile); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func loadServiceConfig(app *application.Avalanche) (*service.Config, error) {
	svcFile := filepath.Join(app.GetServiceDir(), constants.ServiceFile)
	content, err := os.ReadFile(svcFile)
	if err != nil {
		return nil, err
	}
	var cfg *service.Config
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeServiceConfig(app *application.Avalanche, cfg *service.Config) error {
	svcFile := filepath.Join(app.GetServiceDir(), constants.ServiceFile)
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(svcFile, data, constants.DefaultPerms755)
}

package validator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/kardianos/service"
)

type AvalanchegoValidator struct {
	Network  models.Network
	buildDir string
}

func (a *AvalanchegoValidator) Start(svc service.Service) error {
	svc.Start()
	/*
		var network string
		switch a.Network {
		case models.Mainnet:
			network = "mainnet"
		case models.Fuji:
			network = "fuji"
			// the usefulness of this is uncertain at this point,
			// it's debatable if running a local node like this makes sense
		case models.Local:
			network = "1337"
		default:
			return fmt.Errorf("unsupported network type")
		}
	*/

	/*
		fs := config.BuildFlagSet()
		v, err := config.BuildViper(fs, args)
		runnerConfig, err := config.GetRunnerConfig(v)
		if err != nil {
			fmt.Printf("couldn't load process config: %s\n", err)
			os.Exit(1)
		}

		nodeConfig, err := config.GetNodeConfig(v, runnerConfig.BuildDir)
		if err != nil {
			fmt.Printf("couldn't load node config: %s\n", err)
			os.Exit(1)
		}

		runner.Run(runnerConfig, nodeConfig)
	*/
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
	fmt.Println(bin)
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
	/*
		if err := svc.Install(); err != nil {
			return err
		}
	*/
	ux.Logger.PrintToUser("starting a new avalanchego node for network %q as system service...", network.String())
	/*
		if err := svc.Run(); err != nil {
			return err
		}
	*/
	if err := service.Control(svc, "start"); err != nil {
		return err
	}
	/*
		logger, err = svc.Logger(nil)
		if err != nil {
			log.Fatal(err)
		}
	*/
	ux.Logger.PrintToUser("node started, you can now use your system's service management to check on the node")
	return nil
}

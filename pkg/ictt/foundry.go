// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

var (
	foundryVersion   = "v0.2.1"
	foundryupPath    = utils.ExpandHome("~/.foundry/bin/foundryup")
	defaultForgePath = utils.ExpandHome("~/.foundry/bin/forge")
)

func FoundryIsInstalled() bool {
	_, err := GetForgePath()
	return err == nil
}

func GetForgePath() (string, error) {
	if utils.FileExists(defaultForgePath) {
		return defaultForgePath, nil
	}
	out, err := exec.Command("which", "forge").CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	return "", fmt.Errorf("forge is not installed")
}

func InstallFoundry() error {
	ux.Logger.PrintToUser("Installing Foundry")
	downloadCmd := exec.Command(
		"curl",
		"-L",
		fmt.Sprintf("https://raw.githubusercontent.com/ava-labs/foundry/%s/foundryup/install", foundryVersion),
	)
	cmdsEnv := append(os.Environ(), "XDG_CONFIG_HOME=")
	installCmd := exec.Command("bash")
	installCmd.Env = cmdsEnv
	var downloadOutbuf, downloadErrbuf strings.Builder
	downloadCmdStdoutPipe, err := downloadCmd.StdoutPipe()
	if err != nil {
		return err
	}
	downloadCmd.Stderr = &downloadErrbuf
	installCmd.Stdin = io.TeeReader(downloadCmdStdoutPipe, &downloadOutbuf)
	var installOutbuf, installErrbuf strings.Builder
	installCmd.Stdout = &installOutbuf
	installCmd.Stderr = &installErrbuf
	if err := installCmd.Start(); err != nil {
		return err
	}
	if err := downloadCmd.Run(); err != nil {
		if downloadOutbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadOutbuf.String(), "\n")) //nolint:govet
		}
		if downloadErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadErrbuf.String(), "\n")) //nolint:govet
		}
		return err
	}
	if err := installCmd.Wait(); err != nil {
		if installOutbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n")) //nolint:govet
		}
		if installErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installErrbuf.String(), "\n")) //nolint:govet
		}
		ux.Logger.PrintToUser("installation failed: %s", err.Error())
		return err
	}
	ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n")) //nolint:govet
	foundryupCmd := exec.Command(foundryupPath, "-v", foundryVersion)
	foundryupCmd.Env = cmdsEnv
	out, err := foundryupCmd.CombinedOutput()
	ux.Logger.PrintToUser(string(out)) //nolint:govet
	if err != nil {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Foundry toolset is not available and couldn't automatically be installed. It is a necessary dependency for CLI to compile smart contracts.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://book.getfoundry.sh/getting-started/installation and try again")
		ux.Logger.PrintToUser("")
	}
	return err
}

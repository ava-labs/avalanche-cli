// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"io"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

var (
	foundryupPath = utils.ExpandHome("~/.foundry/bin/foundryup")
	forgePath     = utils.ExpandHome("~/.foundry/bin/forge")
)

func FoundryIsInstalled() bool {
	return utils.IsExecutable(forgePath)
}

func InstallFoundry() error {
	ux.Logger.PrintToUser("Installing Foundry")
	downloadCmd := exec.Command("curl", "-L", "https://foundry.paradigm.xyz")
	installCmd := exec.Command("sh")
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
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadOutbuf.String(), "\n"))
		}
		if downloadErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadErrbuf.String(), "\n"))
		}
		return err
	}
	if err := installCmd.Wait(); err != nil {
		if installOutbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n"))
		}
		if installErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installErrbuf.String(), "\n"))
		}
		ux.Logger.PrintToUser("installation failed: %s", err.Error())
		return err
	}
	ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n"))
	out, err := exec.Command(foundryupPath).CombinedOutput()
	ux.Logger.PrintToUser(string(out))
	if err != nil {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Foundry toolset is not available and couldn't automatically be installed. It is a necessary dependency for CLI to compile smart contracts.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://book.getfoundry.sh/getting-started/installation and try again")
		ux.Logger.PrintToUser("")
	}
	return err
}

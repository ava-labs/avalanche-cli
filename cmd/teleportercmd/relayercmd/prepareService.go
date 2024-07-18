// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/spf13/cobra"
)

//go:embed awm-relayer.service
var awmRelayerServiceTemplate []byte

// avalanche teleporter msg
func newPrepareServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepareService",
		Short: "Installs AWM relayer as a service",
		Long:  `Installs AWM relayer as a service. Disabled by default.`,
		RunE:  prepareService,
		Args:  cobrautils.ExactArgs(0),
	}
	return cmd
}

func prepareService(_ *cobra.Command, _ []string) error {
	relayerBin, err := teleporter.InstallRelayer(app.GetAWMRelayerBinDir())
	if err != nil {
		return err
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(app.GetAWMRelayerServiceDir(""), constants.DefaultPerms755); err != nil {
		return err
	}
	awmRelayerServicePath := filepath.Join(app.GetAWMRelayerServiceDir(""), "awm-relayer.service")
	awmRelayerServiceConf := fmt.Sprintf(string(awmRelayerServiceTemplate), usr.Username, usr.HomeDir, relayerBin, app.GetAWMRelayerServiceConfigPath(""))
	if err := os.WriteFile(awmRelayerServicePath, []byte(awmRelayerServiceConf), constants.WriteReadReadPerms); err != nil {
		return err
	}
	return os.RemoveAll(app.GetAWMRelayerServiceConfigPath(""))
}

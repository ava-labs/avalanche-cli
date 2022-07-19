// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/validator"
	"github.com/spf13/cobra"
)

func newStartCmd(injectedApp *application.Avalanche) *cobra.Command {
	return &cobra.Command{
		Use:   "start [subnet]",
		Short: "Starts a validator",
		Long: `The network start command starts a local, multi-node Avalanche network
on your machine. If "snapshotName" is provided, that snapshot will be used for starting the network 
if it can be found. Otherwise, the last saved unnamed (default) snapshot will be used. The command may fail if the local network
is already running or if no subnets have been deployed.`,

		RunE:         startValidator,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
}

func startValidator(cmd *cobra.Command, args []string) error {
	d := subnet.NewLocalSubnetDeployer(app)
	avagoBinDir, _, err := d.SetupLocalEnv()
	if err != nil {
		return err
	}
	if err := validator.StartLocalNodeAsService(models.Fuji, avagoBinDir); err != nil {
		return err
	}
	return nil
}

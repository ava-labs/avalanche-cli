// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/validator"
	"github.com/spf13/cobra"
)

var validatorStartCmd = &cobra.Command{
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

func startValidator(cmd *cobra.Command, args []string) error {
	/*
		chain := args[0]
		chain_genesis := filepath.Join(app.GetBaseDir(), fmt.Sprintf("%s_genesis.json", chain))
		deployer := subnet.NewFujiSubnetDeployer(app)
		if err := deployer.StartValidator(chain, chain_genesis, subnet.Local); err != nil {
			return err
		}
	*/
	d := subnet.NewLocalSubnetDeployer(app)
	avagoBinDir, err := d.SetupLocalEnv()
	if err != nil {
		return err
	}
	if err := validator.StartLocalNodeAsService(models.Fuji, avagoBinDir); err != nil {
		return err
	}
	return nil
}

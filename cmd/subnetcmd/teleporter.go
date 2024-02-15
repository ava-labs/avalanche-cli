// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"

	"github.com/spf13/cobra"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "teleporter",
		Short:             "Deploys teleporter to local network cchain",
		Long:              `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	return teleporter.DeployAWMRelayer(app, "v0.2.12")
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

const (
	cChainRpcURL            = constants.LocalAPIEndpoint + "/ext/bc/C/rpc"
	prefundedEwoqPrivateKey = "0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
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
	if b, err := teleporter.TeleporterAlreadyDeployed(cChainRpcURL); err != nil {
		return err
	} else if b {
		ux.Logger.PrintToUser("Teleporter has already been deployed to c-chain")
		return nil
	}
	if err := teleporter.DeployTeleporter(cChainRpcURL, prefundedEwoqPrivateKey); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Teleporter successfully deployed to c-chain")
	return nil
}

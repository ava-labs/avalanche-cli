// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validator"
	"github.com/spf13/cobra"
)

func newStatusCmd(injectedApp *application.Avalanche) *cobra.Command {
	return &cobra.Command{
		Use:   "status [subnet]",
		Short: "Starts a validator",
		Long: `The network start command starts a local, multi-node Avalanche network
on your machine. If "snapshotName" is provided, that snapshot will be used for starting the network 
if it can be found. Otherwise, the last saved unnamed (default) snapshot will be used. The command may fail if the local network
is already running or if no subnets have been deployed.`,

		RunE:         validatorStatus,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
}

func validatorStatus(cmd *cobra.Command, args []string) error {
	status, err := validator.GetStatus(app)
	if err != nil {
		return err
	}
	if strings.Contains(status, "Active: inactive") {
		ux.Logger.PrintToUser("not running")
	} else if strings.Contains(status, "Active: active (running)") {
		ux.Logger.PrintToUser("running")
	} else {
		ux.Logger.PrintToUser("status unknown")
	}
	return nil
}

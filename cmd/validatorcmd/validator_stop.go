// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/validator"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops a validator",
		Long:  `Stops a running validator. If it is not running, does nothing`,

		RunE:         stopValidator,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
}

func stopValidator(cmd *cobra.Command, args []string) error {
	return validator.StopLocalNodeAsService(models.Fuji, "", app)
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// avalanche config singlenode command
func newSingleNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "singleNode [enable | disable]",
		Short: "opt in or out of single node local network",
		Long:  "set user preference between single node and five nodes local network",
		RunE: func(_ *cobra.Command, args []string) error {
			return handleBooleanSetting(constants.ConfigSingleNodeEnabledKey, args)
		},
		Args: cobrautils.ExactArgs(1),
	}

	return cmd
}

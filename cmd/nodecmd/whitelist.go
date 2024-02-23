// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "whitelist <clusterName> [IP]",
		Short:        "(DEPRICATED) Please use node whitelist-ip command instead.",
		Long:         `(DEPRICATED) Please use node whitelist-ip command instead.`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         whitelist,
	}
	return cmd
}

func whitelist(_ *cobra.Command, _ []string) error {
	ux.Logger.PrintToUser("This command is depricated. Please use node whitelist-ip command instead.")
	return nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func newWhitelistCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "(ALPHA Warning) Grant access to the cluster ",
		Long:  `(ALPHA Warning) The whitelist command suite provides a collection of tools for granting access to the cluster.`,
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	app = injectedApp
	// whitelist ip
	cmd.AddCommand(newWhitelistIPCmd())
	// whitelist pubkey
	cmd.AddCommand(newWhitelistSSHCmd())
	return cmd
}

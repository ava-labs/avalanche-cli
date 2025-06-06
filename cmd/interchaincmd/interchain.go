// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package interchaincmd

import (
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/messengercmd"
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/relayercmd"
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/tokentransferrercmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interchain",
		Short: "Set and manage interoperability between blockchains",
		Long: `The interchain command suite provides a collection of tools to
set and manage interoperability between blockchains.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// interchain tokenTransferrer
	cmd.AddCommand(tokentransferrercmd.NewCmd(app))
	// interchain relayer
	cmd.AddCommand(relayercmd.NewCmd(app))
	// interchain messenger
	cmd.AddCommand(messengercmd.NewCmd(app))
	return cmd
}

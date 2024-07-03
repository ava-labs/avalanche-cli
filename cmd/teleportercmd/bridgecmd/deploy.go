// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"

	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/tokentransferrercmd"

	"github.com/spf13/cobra"
)

// avalanche teleporter bridge deploy
func newDeployCmd() *cobra.Command {
	cmd := tokentransferrercmd.NewDeployCmd()
	cmd.Use = "deploy"
	cmd.Short = "Deploys Token Bridge into a given Network and Subnets (deprecation notice: use 'avalanche interchain tokenTransferrer deploy')"
	cmd.Long = `Deploys Token Bridge into a given Network and Subnets

Deprecation notice: use 'avalanche interchain tokenTransferrer deploy`
	return cmd
}

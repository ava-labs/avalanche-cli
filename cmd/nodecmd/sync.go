// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Blockchain.
You can check the blockchain bootstrap status by calling avalanche node status <clusterName> --blockchain <blockchainName>`,
		Args: cobrautils.ExactArgs(2),
		RunE: syncSubnet,
	}

	cmd.Flags().StringSliceVar(&validators, "validators", []string{}, "sync subnet into given comma separated list of validators. defaults to all cluster nodes")
	cmd.Flags().BoolVar(&avoidChecks, "no-checks", false, "do not check for bootstrapped/healthy status or rpc compatibility of nodes against subnet")
	cmd.Flags().StringSliceVar(&subnetAliases, "subnet-aliases", nil, "subnet alias to be used for RPC calls. defaults to subnet blockchain ID")

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	blockchainName := args[1]
	return node.SyncSubnet(app, clusterName, blockchainName, avoidChecks, subnetAliases)
}

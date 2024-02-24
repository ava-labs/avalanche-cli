// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"github.com/spf13/cobra"
)

var (
	useLocal        bool
	useDevnet       bool
	useTestnet      bool
	useMainnet      bool
	endpoint string
)

// avalanche teleporter msg
func newMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "msg [subnet1Name] [subnet2Name]",
		Short:        "Sends and wait reception for a teleporter msg between two subnets",
		Long:         `Sends and wait reception for a teleporter msg between two subnets.`,
		SilenceUsage: true,
		RunE:         msg,
		Args:         cobra.ExactArgs(2),
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "use the given endpoint for network operations")
	cmd.Flags().BoolVarP(&useLocal, "local", "l", false, "operate on a local network")
	cmd.Flags().BoolVar(&useDevnet, "devnet", false, "operate on a devnet network")
	cmd.Flags().BoolVarP(&useTestnet, "testnet", "t", false, "operate on testnet (alias to `fuji`)")
	cmd.Flags().BoolVarP(&useTestnet, "fuji", "f", false, "operate on fuji (alias to `testnet`")
	cmd.Flags().BoolVarP(&useMainnet, "mainnet", "m", false, "operate on mainnet")
	return cmd
}

func msg(cmd *cobra.Command, args []string) error {
	return nil
}

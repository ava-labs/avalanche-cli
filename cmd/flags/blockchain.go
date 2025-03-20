// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/spf13/cobra"
)

const (
	rpcURLFLag = "rpc"
)

var (
	RPC string
)

func AddRPCFlagToCmd(cmd *cobra.Command) {
	cmd.Flags().StringVar(&RPC, rpcURLFLag, "", "blockchain rpc endpoint")
}

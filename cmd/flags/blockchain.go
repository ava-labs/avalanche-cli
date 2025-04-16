// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/spf13/cobra"
)

const (
	rpcURLFLag = "rpc"
)

func AddRPCFlagToCmd(cmd *cobra.Command, app *application.Avalanche, rpc *string) {
	cmd.Flags().StringVar(rpc, rpcURLFLag, "", "blockchain rpc endpoint")

	rpcPreRun := func(cmd *cobra.Command, args []string) error {
		if err := ValidateRPC(app, rpc, cmd, args); err != nil {
			return err
		}
		return nil
	}

	existingPreRunE := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if existingPreRunE != nil {
			if err := existingPreRunE(cmd, args); err != nil {
				return err
			}
		}
		return rpcPreRun(cmd, args)
	}
}

func ValidateRPC(app *application.Avalanche, rpc *string, cmd *cobra.Command, args []string) error {
	var err error
	// TODO: modify check below to extend prompting for rpc to commands other than addValidator
	if *rpc == "" {
		if cmd.Name() == "addValidator" && len(args) == 0 {
			*rpc, err = app.Prompt.CaptureURL("What is the RPC endpoint?", false)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return prompts.ValidateURLFormat(*rpc)
}

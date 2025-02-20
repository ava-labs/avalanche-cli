// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche blockchain vmid
func vmidCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vmid [vmName]",
		Short: "Prints the VMID of a VM",
		Long:  `The blockchain vmid command prints the virtual machine ID (VMID) for the given Blockchain.`,
		Args:  cobrautils.ExactArgs(1),
		RunE:  printVMID,
	}
	return cmd
}

func printVMID(_ *cobra.Command, args []string) error {
	chains, err := ValidateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}

	chain := chains[0]
	vmID, err := utils.VMID(chain)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser(fmt.Sprintf("VM ID : %s", vmID.String()))
	return nil
}

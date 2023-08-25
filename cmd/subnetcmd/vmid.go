// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/spf13/cobra"
)

// avalanche subnet create
func vmidCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "vmid [vmName]",
		Short:        "Prints the VMID of a VM",
		Long:         `The subnet vmid command prints the virtual machine ID (VMID) for the given Subnet.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         printVMID,
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

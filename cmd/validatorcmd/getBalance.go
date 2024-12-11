// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	subnetID string
	nodeID   string
)

func NewGetBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "getBalance",
		Short: "Gets current balance of validator on P-Chain",
		Long: `This command gets the remaining validator P-Chain balance that is available to pay
P-Chain continuous fee`,
		RunE: getBalance,
		Args: cobrautils.ExactArgs(0),
	}

	cmd.Flags().StringVar(&subnetID, "subnet-id", "", "subnetID of L1 that the node is validating")
	cmd.Flags().StringVar(&nodeID, "node-id", "", "Node-ID of validator")
	return cmd
}

func getBalance(cmd *cobra.Command, _ []string) error {
	subnetID, err := ids.FromString("d")
	if err != nil {
		return err
	}
	index := uint32(0)
	network := models.NewFujiNetwork()
	err := txutils.GetBalance(network, subnetID, index)
	if err != nil {
		return err
	}
	return nil
}

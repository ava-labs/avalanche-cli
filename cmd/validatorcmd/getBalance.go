// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"errors"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var (
	ErrUserAbortedInstallation = errors.New("user canceled installation")
	ErrNoVersion               = errors.New("failed to find current version - did you install following official instructions?")
	app                        *application.Avalanche
	yes                        bool
)

func NewGetBalanceCmd(injectedApp *application.Avalanche, version string) *cobra.Command {
	app = injectedApp
	cmd := &cobra.Command{
		Use:   "getBalance",
		Short: "Gets current balance of validator on P-Chain",
		Long: `Validator's balance is used to pay for continuous fee to the P-Chain. 
When this Balance reaches 0, the validator will be considered inactive and will no longer participate in validating the L1.
	This command gets the remaining validator P-Chain balance`,
		RunE:    getBalance,
		Args:    cobrautils.ExactArgs(0),
		Version: version,
	}

	cmd.Flags().BoolVarP(&yes, "node-id", "c", false, "Assume yes for installation")
	return cmd
}

func getBalance(cmd *cobra.Command, _ []string) error {
	return nil
}

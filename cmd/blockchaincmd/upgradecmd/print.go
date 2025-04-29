// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"bytes"
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche blockchain upgrade print
func newUpgradePrintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print [blockchainName]",
		Short: "Print the upgrade.json file content",
		Long:  `Print the upgrade.json file content`,
		RunE:  upgradePrintCmd,
		Args:  cobrautils.ExactArgs(1),
	}

	return cmd
}

func upgradePrintCmd(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	if !app.GenesisExists(blockchainName) {
		ux.Logger.PrintToUser("The provided blockchain name %q does not exist", blockchainName)
		return nil
	}

	fileBytes, err := app.ReadUpgradeFile(blockchainName)
	if err != nil {
		return err
	}

	var prettyJSON bytes.Buffer
	if err = json.Indent(&prettyJSON, fileBytes, "", "  "); err != nil {
		return err
	}
	ux.Logger.PrintToUser(prettyJSON.String())
	return nil
}

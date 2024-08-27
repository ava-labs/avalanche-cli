// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/spf13/cobra"
)

type PrivateKeyFlags struct {
	PrivateKey string
	KeyName    string
	GenesisKey bool
}

func AddPrivateKeyFlagsToCmd(
	cmd *cobra.Command,
	privateKeyFlags *PrivateKeyFlags,
	goal string,
) {
	cmd.Flags().StringVar(
		&privateKeyFlags.PrivateKey,
		"private-key",
		"",
		fmt.Sprintf("private key to use %s", goal),
	)
	cmd.Flags().StringVar(
		&privateKeyFlags.KeyName,
		"key",
		"",
		fmt.Sprintf("CLI stored key to use %s", goal),
	)
	cmd.Flags().BoolVar(
		&privateKeyFlags.GenesisKey,
		"genesis-key",
		false,
		fmt.Sprintf("use genesis allocated key %s", goal),
	)
}

func GetPrivateKeyFromFlags(
	app *application.Avalanche,
	flags PrivateKeyFlags,
	genesisPrivateKey string,
) (string, error) {
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		flags.PrivateKey != "",
		flags.KeyName != "",
		flags.GenesisKey,
	}) {
		return "", fmt.Errorf("--private-key, --key and --genesis-key are mutually exclusive flags")
	}
	privateKey := flags.PrivateKey
	if flags.KeyName != "" {
		k, err := app.GetKey(flags.KeyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	}
	if flags.GenesisKey {
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

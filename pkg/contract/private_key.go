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
	PrivateKeyFlagName string
	PrivateKey         string
	KeyFlagName        string
	KeyName            string
	GenesisKeyFlagName string
	GenesisKey         bool
}

const (
	defaultPrivateKeyFlagName = "private-key"
	defaultKeyFlagName        = "key"
	defaultGenesisKeyFlagName = "genesis-key"
)

func (pkf *PrivateKeyFlags) fillDefaultFlagNames() {
	if pkf.PrivateKeyFlagName == "" {
		pkf.PrivateKeyFlagName = defaultPrivateKeyFlagName
	}
	if pkf.KeyFlagName == "" {
		pkf.KeyFlagName = defaultKeyFlagName
	}
	if pkf.GenesisKeyFlagName == "" {
		pkf.GenesisKeyFlagName = defaultGenesisKeyFlagName
	}
}

func (pkf *PrivateKeyFlags) SetFlagNames(
	privateKeyFlagName string,
	keyFlagName string,
	genesisKeyFlagName string,
) {
	pkf.PrivateKeyFlagName = privateKeyFlagName
	pkf.KeyFlagName = keyFlagName
	pkf.GenesisKeyFlagName = genesisKeyFlagName
}

func (pkf *PrivateKeyFlags) AddToCmd(
	cmd *cobra.Command,
	goal string,
) {
	pkf.fillDefaultFlagNames()
	cmd.Flags().StringVar(
		&pkf.PrivateKey,
		pkf.PrivateKeyFlagName,
		"",
		fmt.Sprintf("private key to use %s", goal),
	)
	cmd.Flags().StringVar(
		&pkf.KeyName,
		pkf.KeyFlagName,
		"",
		fmt.Sprintf("CLI stored key to use %s", goal),
	)
	cmd.Flags().BoolVar(
		&pkf.GenesisKey,
		pkf.GenesisKeyFlagName,
		false,
		fmt.Sprintf("use genesis allocated key %s", goal),
	)
}

func (pkf *PrivateKeyFlags) GetPrivateKey(
	app *application.Avalanche,
	genesisPrivateKey string,
) (string, error) {
	pkf.fillDefaultFlagNames()
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		pkf.PrivateKey != "",
		pkf.KeyName != "",
		pkf.GenesisKey,
	}) {
		return "", fmt.Errorf("%s, %s and %s are mutually exclusive flags",
			pkf.PrivateKeyFlagName,
			pkf.KeyFlagName,
			pkf.GenesisKeyFlagName,
		)
	}
	privateKey := pkf.PrivateKey
	if pkf.KeyName != "" {
		k, err := app.GetKey(pkf.KeyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	}
	if pkf.GenesisKey {
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

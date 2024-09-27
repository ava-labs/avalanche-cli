// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
)

type PrivateKeyFlags struct {
	privateKeyFlagName string
	keyFlagName        string
	genesisKeyFlagName string
	PrivateKey         string
	KeyName            string
	GenesisKey         bool
}

const (
	defaultPrivateKeyFlagName = "private-key"
	defaultKeyFlagName        = "key"
	defaultGenesisKeyFlagName = "genesis-key"
)

func (pkf *PrivateKeyFlags) fillDefaultFlagNames() {
	if pkf.privateKeyFlagName == "" {
		pkf.privateKeyFlagName = defaultPrivateKeyFlagName
	}
	if pkf.keyFlagName == "" {
		pkf.keyFlagName = defaultKeyFlagName
	}
	if pkf.genesisKeyFlagName == "" {
		pkf.genesisKeyFlagName = defaultGenesisKeyFlagName
	}
}

func (pkf *PrivateKeyFlags) SetFlagNames(
	privateKeyFlagName string,
	keyFlagName string,
	genesisKeyFlagName string,
) {
	pkf.privateKeyFlagName = privateKeyFlagName
	pkf.keyFlagName = keyFlagName
	pkf.genesisKeyFlagName = genesisKeyFlagName
}

func (pkf *PrivateKeyFlags) AddToCmd(
	cmd *cobra.Command,
	goal string,
) {
	pkf.fillDefaultFlagNames()
	cmd.Flags().StringVar(
		&pkf.PrivateKey,
		pkf.privateKeyFlagName,
		"",
		fmt.Sprintf("private key to use %s", goal),
	)
	cmd.Flags().StringVar(
		&pkf.KeyName,
		pkf.keyFlagName,
		"",
		fmt.Sprintf("CLI stored key to use %s", goal),
	)
	cmd.Flags().BoolVar(
		&pkf.GenesisKey,
		pkf.genesisKeyFlagName,
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
			pkf.privateKeyFlagName,
			pkf.keyFlagName,
			pkf.genesisKeyFlagName,
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

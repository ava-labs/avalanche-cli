// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/validator"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Installs a validator",
		Long:  `Installs the validator binary as a system service, allowing management of the binary via OS service management`,

		RunE:         installValidator,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
}

func installValidator(cmd *cobra.Command, args []string) error {
	d := subnet.NewLocalSubnetDeployer(app)
	avagoBinDir, _, err := d.SetupLocalEnv()
	if err != nil {
		return err
	}
	return validator.InstallAsAService(models.Fuji, avagoBinDir, app)
}

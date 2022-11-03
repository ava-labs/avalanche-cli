// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

// avalanche subnet configure
func newConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "configure [subnetName]",
		Short:        "",
		Long:         ``,
		SilenceUsage: true,
		RunE:         configure,
		Args:         cobra.ExactArgs(1),
	}
	return cmd
}

func configure(cmd *cobra.Command, args []string) error {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return err
	}
	subnetName := chains[0]

	const (
		subnetConf = "Subnet config"
		chainConf  = "Chain config"
		abort      = "Abort"
	)

	ux.Logger.PrintToUser("The " + logging.Cyan.Wrap("subnet") + logging.Reset.Wrap(" config file applies to *all* VMs in a subnet"))
	ux.Logger.PrintToUser("The " + logging.Cyan.Wrap("chain") + logging.Reset.Wrap(" config file applies to a *specific* VM in a subnet"))

	options := []string{subnetConf, chainConf, abort}
	selected, err := app.Prompt.CaptureList("Which configuration file would you like to update?", options)
	if err != nil {
		return err
	}
	switch selected {
	case abort:
		ux.Logger.PrintToUser("Aborted by user. Nothing changed.")
		return nil
	case subnetConf:
		err = updateConf(subnetName, constants.SubnetConfigFileName)
	case chainConf:
		err = updateConf(subnetName, constants.ChainConfigFileName)
	}
	if err != nil {
		return err
	}

	return nil
}

func updateConf(subnet, filename string) error {
	path, err := app.Prompt.CaptureExistingFilepath("Where can we find the configuration file?")
	if err != nil {
		return err
	}
	fileBytes, err := validateJSON(path)
	if err != nil {
		return err
	}
	subnetDir := filepath.Join(app.GetBaseDir(), subnet)
	if err := os.MkdirAll(subnetDir, constants.DefaultPerms755); err != nil {
		return err
	}
	fileName := filepath.Join(subnetDir, filename)
	if err := os.WriteFile(fileName, fileBytes, constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File %s successfully written", fileName)

	return nil
}

func validateJSON(path string) ([]byte, error) {
	var content map[string]interface{}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// if the file is not valid json, this fails
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return nil, fmt.Errorf("this looks like invalid JSON: %w", err)
	}

	return contentBytes, nil
}

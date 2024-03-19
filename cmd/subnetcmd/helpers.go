// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/spf13/cobra"
)

var globalNetworkFlags networkoptions.NetworkFlags

func CreateSubnetFirst(cmd *cobra.Command, subnetName string, skipPrompt bool) error {
	if !app.SubnetConfigExists(subnetName) {
		if !skipPrompt {
			yes, err := app.Prompt.CaptureNoYes(fmt.Sprintf("Subnet %s is not created yet. Do you want to create it first?", subnetName))
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("subnet not available and not being created first")
			}
		}
		return createSubnetConfig(cmd, []string{subnetName})
	}
	return nil
}

func DeploySubnetFirst(cmd *cobra.Command, subnetName string, skipPrompt bool, supportedNetworkOptions []networkoptions.NetworkOption) error {
	var (
		doDeploy       bool
		msg            string
		errIfNoChoosen error
	)
	if !app.SubnetConfigExists(subnetName) {
		doDeploy = true
		msg = fmt.Sprintf("Subnet %s is not created yet. Do you want to create it first?", subnetName)
		errIfNoChoosen = fmt.Errorf("subnet not available and not being created first")
	} else {
		filteredSupportedNetworkOptions, _, _, err := networkoptions.GetSupportedNetworkOptionsForSubnet(app, subnetName, supportedNetworkOptions)
		if err != nil {
			return err
		}
		if len(filteredSupportedNetworkOptions) == 0 {
			doDeploy = true
			msg = fmt.Sprintf("Subnet %s is not deployed yet to a supported network. Do you want to deploy it first?", subnetName)
			errIfNoChoosen = fmt.Errorf("subnet not deployed and not being deployed first")
		}
	}
	if doDeploy {
		if !skipPrompt {
			yes, err := app.Prompt.CaptureNoYes(msg)
			if err != nil {
				return err
			}
			if !yes {
				return errIfNoChoosen
			}
		}
		return runDeploy(cmd, []string{subnetName}, supportedNetworkOptions)
	}
	return nil
}

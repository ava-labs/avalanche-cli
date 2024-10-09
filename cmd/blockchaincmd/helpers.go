// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var globalNetworkFlags networkoptions.NetworkFlags

func CreateBlockchainFirst(cmd *cobra.Command, blockchainName string, skipPrompt bool) error {
	if !app.BlockchainConfigExists(blockchainName) {
		if !skipPrompt {
			yes, err := app.Prompt.CaptureNoYes(fmt.Sprintf("Blockchain %s is not created yet. Do you want to create it first?", blockchainName))
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("blockchain not available and not being created first")
			}
		}
		return createBlockchainConfig(cmd, []string{blockchainName})
	}
	return nil
}

func DeployBlockchainFirst(cmd *cobra.Command, blockchainName string, skipPrompt bool, supportedNetworkOptions []networkoptions.NetworkOption) error {
	var (
		doDeploy       bool
		msg            string
		errIfNoChoosen error
	)
	if !app.BlockchainConfigExists(blockchainName) {
		doDeploy = true
		msg = fmt.Sprintf("Blockchain %s is not created yet. Do you want to create it first?", blockchainName)
		errIfNoChoosen = fmt.Errorf("blockchain not available and not being created first")
	} else {
		filteredSupportedNetworkOptions, _, _, err := networkoptions.GetSupportedNetworkOptionsForSubnet(app, blockchainName, supportedNetworkOptions)
		if err != nil {
			return err
		}
		if len(filteredSupportedNetworkOptions) == 0 {
			doDeploy = true
			msg = fmt.Sprintf("Blockchain %s is not deployed yet to a supported network. Do you want to deploy it first?", blockchainName)
			errIfNoChoosen = fmt.Errorf("blockchain not deployed and not being deployed first")
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
		return runDeploy(cmd, []string{blockchainName}, supportedNetworkOptions)
	}
	return nil
}

func UpdateKeychainWithSubnetControlKeys(
	kc *keychain.Keychain,
	network models.Network,
	blockchainName string,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}
	isPermissioned, controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	if !isPermissioned {
		return ErrNotPermissionedSubnet
	}
	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}
	return nil
}

func getClusterNameFromList() (string, error) {
	clusterNames, err := app.ListClusterNames()
	if err != nil {
		return "", err
	}
	if len(clusterNames) == 0 {
		return "", fmt.Errorf("no Avalanche nodes found that can track the blockchain, please create Avalanche nodes first through `avalanche node create`")
	}
	clusterName, err := app.Prompt.CaptureList(
		"Which cluster of Avalanche nodes would you like to use to track the blockchain?",
		clusterNames,
	)
	if err != nil {
		return "", err
	}
	return clusterName, nil
}

// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ethereum/go-ethereum/common"
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

func DeployBlockchainFirst(cmd *cobra.Command, blockchainName string, skipPrompt bool) error {
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
		filteredSupportedNetworkOptions, _, _, err := networkoptions.GetSupportedNetworkOptionsForSubnet(app, blockchainName, networkoptions.DefaultSupportedNetworkOptions)
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
		return runDeploy(cmd, []string{blockchainName})
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
	_, controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}
	return nil
}

func getLocalBootstrapEndpoints() ([]string, error) {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	serverEndpoint := binutils.LocalClusterGRPCServerEndpoint
	cli, err := binutils.NewGRPCClientWithEndpoint(serverEndpoint)
	if err != nil {
		return nil, err
	}
	status, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}
	localBootstrapEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		localBootstrapEndpoints = append(localBootstrapEndpoints, nodeInfo.Uri)
	}
	return localBootstrapEndpoints, nil
}

func GetProxyOwnerPrivateKey(
	app *application.Avalanche,
	network models.Network,
	proxyContractOwner string,
	printFunc func(msg string, args ...interface{}),
) (string, error) {
	found, _, _, proxyOwnerPrivateKey, err := contract.SearchForManagedKey(
		app,
		network,
		common.HexToAddress(proxyContractOwner),
		true,
	)
	if err != nil {
		return "", err
	}
	if !found {
		printFunc("Private key for proxy owner address %s was not found", proxyContractOwner)
		proxyOwnerPrivateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"configure validator manager proxy for PoS",
			app.GetKeyDir(),
			app.GetKey,
			"",
			"",
		)
		if err != nil {
			return "", err
		}
	}
	return proxyOwnerPrivateKey, nil
}

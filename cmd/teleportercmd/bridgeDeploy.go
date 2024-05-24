// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	_ "embed"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type BridgeDeployFlags struct {
	Network           networkoptions.NetworkFlags
	SubnetName        string
	BlockchainID      string
	CChain            bool
	PrivateKey        string
	KeyName           string
	GenesisKey        bool
	DeployMessenger   bool
	DeployRegistry    bool
	TeleporterVersion string
	RPCURL            string
}

var (
	bridgeDeploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	bridgeDeployFlags BridgeDeployFlags
)

// avalanche teleporter bridge deploy
func newBridgeDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys Token Bridge into a given Network and Subnets",
		Long:  "Deploys Token Bridge into a given Network and Subnets",
		RunE:  bridgeDeploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, bridgeDeploySupportedNetworkOptions)
	return cmd
}

func bridgeDeploy(_ *cobra.Command, args []string) error {
	return CallBridgeDeploy(args, deployFlags)
}

func CallBridgeDeploy(_ []string, flags DeployFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter bridge?",
		flags.Network,
		true,
		false,
		bridgeDeploySupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	switch network.Kind {
	case models.Local:
		ux.Logger.PrintToUser("To be defined")
		return nil
	case models.Devnet:
		ux.Logger.PrintToUser("To be defined")
		return nil
	case models.Fuji:
		subnetNames, err := app.GetSubnetNames()
		if err != nil {
			return err
		}
		subnetOptions := utils.Map(subnetNames, func(s string) string { return "Subnet " + s })
		prompt := "Where is the Token origin?"
		cChainOption := "C-Chain"
		notListedOption := "My blockchain isn't listed"
		subnetOptions = append(append([]string{cChainOption}, subnetOptions...), notListedOption)
		subnetOption, err := app.Prompt.CaptureListWithSize(
			prompt,
			subnetOptions,
			11,
		)
		if err != nil {
			return err
		}
		if subnetOption == notListedOption {
			ux.Logger.PrintToUser("Please import the subnet first, using the `avalanche subnet import` command suite")
			return nil
		}
		tokenSymbol := "AVAX"
		if subnetOption != cChainOption {
			subnetName := strings.TrimPrefix(subnetOption, "Subnet ")
			sc, err := app.LoadSidecar(subnetName)
			if err != nil {
				return err
			}
			tokenSymbol = sc.TokenSymbol
		}
		prompt = "What kind of token do you want to bridge?"
		popularOption := "A popular token (e.g. AVAX, USDC, WAVAX, ...)"
		existingOriginOption := "A token with an existing Origin Bridge"
		nativeOption := "The native token " + tokenSymbol
		erc20Option := "An ERC-20 token"
		explainOption := "Explain the difference"
		popularTokensInfo, err := GetPopularTokensInfo(network, subnetOption)
		if err != nil {
			return err
		}
		popularTokensDesc := utils.Map(
			popularTokensInfo,
			func(i PopularTokenInfo) string {
				return i.Desc()
			},
		)
		options := []string{popularOption, existingOriginOption, nativeOption, erc20Option, explainOption}
		if len(popularTokensDesc) == 0 {
			options = []string{existingOriginOption, nativeOption, erc20Option, explainOption}
		}
		for {
			option, err := app.Prompt.CaptureList(
				prompt,
				options,
			)
			if err != nil {
				return err
			}
			switch option {
			case popularOption:
				_, err = app.Prompt.CaptureList(
					"Choose Token",
					popularTokensDesc,
				)
				if err != nil {
					return err
				}
			case existingOriginOption:
				_, err = app.Prompt.CaptureAddress(
					"Enter the address of the Origin Bridge",
				)
				if err != nil {
					return err
				}
			case erc20Option:
				_, err = app.Prompt.CaptureAddress(
					"Enter the address of the ERC-20 Token",
				)
				if err != nil {
					return err
				}
			case explainOption:
				ux.Logger.PrintToUser("The difference is...")
				ux.Logger.PrintToUser("")
				continue
			}
			break
		}
		prompt = "Where should the token be bridged as an ERC-20?"
		subnetOptions = utils.Filter(subnetOptions, func(s string) bool { return s != subnetOption })
		subnetOption, err = app.Prompt.CaptureListWithSize(
			prompt,
			subnetOptions,
			11,
		)
		if err != nil {
			return err
		}
		if subnetOption == notListedOption {
			ux.Logger.PrintToUser("Please import the subnet first, using the `avalanche subnet import` command suite")
			return nil
		}
	}
	return nil
}

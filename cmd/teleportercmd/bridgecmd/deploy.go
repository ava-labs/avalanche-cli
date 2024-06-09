// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

type ChainFlags struct {
	SubnetName   string
	BlockchainID string
	CChain       bool
}

type HubFlags struct {
	chainFlags   ChainFlags
	hubAddress   string
	native       bool
	erc20Address string
}

type DeployFlags struct {
	Network           networkoptions.NetworkFlags
	hubFlags          HubFlags
	spokeFlags        ChainFlags
	PrivateKey        string
	KeyName           string
	GenesisKey        bool
	DeployMessenger   bool
	DeployRegistry    bool
	TeleporterVersion string
	RPCURL            string
}

var (
	deploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployFlags DeployFlags
)

// avalanche teleporter bridge deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys Token Bridge into a given Network and Subnets",
		Long:  "Deploys Token Bridge into a given Network and Subnets",
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.hubFlags.chainFlags.SubnetName, "hub-subnet", "", "use the given CLI subnet as the Bridge Hub's Chain")
	cmd.Flags().StringVar(&deployFlags.hubFlags.chainFlags.BlockchainID, "hub-blockchain-id", "", "use the given blockchain ID/Alias as the Bridge Hub's Chain")
	cmd.Flags().BoolVar(&deployFlags.hubFlags.chainFlags.CChain, "c-chain-hub", false, "use C-Chain as the Bridge Hub's Chain")
	cmd.Flags().BoolVar(&deployFlags.hubFlags.native, "deploy-native-hub", false, "deploy a Bridge Hub for the Chain's Native Token")
	cmd.Flags().StringVar(&deployFlags.hubFlags.erc20Address, "deploy-erc20-hub", "", "deploy a Bridge Hub for the Chain's ERC20 Token")
	cmd.Flags().StringVar(&deployFlags.hubFlags.hubAddress, "use-hub", "", "use the given Bridge Hub Address")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(_ []string, flags DeployFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter bridge?",
		flags.Network,
		true,
		false,
		deploySupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if flags.hubFlags.chainFlags.SubnetName == "" && !flags.hubFlags.chainFlags.CChain {
		prompt := "Where is the Token origin?"
		if cancel, err := promptChain(prompt, network, false, "", &flags.hubFlags.chainFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}
	if flags.hubFlags.hubAddress == "" && flags.hubFlags.erc20Address == "" {
		tokenSymbol := "AVAX"
		if !flags.hubFlags.chainFlags.CChain {
			sc, err := app.LoadSidecar(flags.hubFlags.chainFlags.SubnetName)
			if err != nil {
				return err
			}
			tokenSymbol = sc.TokenSymbol
		}
		prompt := "What kind of token do you want to bridge?"
		popularOption := "A popular token (e.g. AVAX, USDC, WAVAX, ...)"
		hubDeployedOption := "A token that already has a Hub deployed"
		deployNewHubOption := "Deploy a new Hub for the token"
		explainOption := "Explain the difference"
		goBackOption := "Go Back"
		hubChain := "C-Chain"
		if !flags.hubFlags.chainFlags.CChain {
			hubChain = flags.hubFlags.chainFlags.SubnetName
		}
		popularTokensInfo, err := GetPopularTokensInfo(network, hubChain)
		if err != nil {
			return err
		}
		popularTokensDesc := utils.Map(
			popularTokensInfo,
			func(i PopularTokenInfo) string {
				if i.BridgeHubAddress == "" {
					return i.Desc()
				} else {
					return i.Desc() + " (recommended)"
				}
			},
		)
		options := []string{popularOption, hubDeployedOption, deployNewHubOption, explainOption}
		if len(popularTokensDesc) == 0 {
			options = []string{hubDeployedOption, deployNewHubOption, explainOption}
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
				options := popularTokensDesc
				options = append(options, goBackOption)
				option, err := app.Prompt.CaptureList(
					"Choose Token",
					options,
				)
				if err != nil {
					return err
				}
				if option == goBackOption {
					continue
				}
			case hubDeployedOption:
				addr, err := app.Prompt.CaptureAddress(
					"Enter the address of the Hub",
				)
				if err != nil {
					return err
				}
				flags.hubFlags.hubAddress = addr.Hex()
			case deployNewHubOption:
				nativeOption := "The native token " + tokenSymbol
				erc20Option := "An ERC-20 token"
				options := []string{nativeOption, erc20Option}
				option, err := app.Prompt.CaptureList(
					"What kind of token do you want to deploy the Hub for?",
					options,
				)
				if err != nil {
					return err
				}
				switch option {
				case nativeOption:
					flags.hubFlags.native = true
				case erc20Option:
					erc20TokenAddr, err := app.Prompt.CaptureAddress(
						"Enter the address of the ERC-20 Token",
					)
					if err != nil {
						return err
					}
					flags.hubFlags.erc20Address = erc20TokenAddr.Hex()
					if p := utils.Find(popularTokensInfo, func(p PopularTokenInfo) bool { return p.TokenContractAddress == erc20TokenAddr.Hex() }); p != nil {
						ux.Logger.PrintToUser("You have entered the address of %s, a popular token in the subnet.", p.TokenName)
						deployANewHupOption := "Yes, I want to deploy a new Bridge Hub"
						useTheExistingHubOption := "No, I want to use the existing official Bridge Hub"
						options := []string{deployANewHupOption, useTheExistingHubOption}
						_, err = app.Prompt.CaptureList(
							"Are you sure you want to deploy a new Bridge Hub for it?",
							options,
						)
						if err != nil {
							return err
						}
					}
				}
			case explainOption:
				ux.Logger.PrintToUser("The difference is...")
				ux.Logger.PrintToUser("")
				continue
			}
			break
		}
	}
	if flags.hubFlags.hubAddress != "" {
		if err := prompts.ValidateAddress(flags.hubFlags.hubAddress); err != nil {
			return fmt.Errorf("failure validating %s: %w", flags.hubFlags.hubAddress, err)
		}
	}
	if flags.hubFlags.erc20Address != "" {
		if err := prompts.ValidateAddress(flags.hubFlags.erc20Address); err != nil {
			return fmt.Errorf("failure validating %s: %w", flags.hubFlags.erc20Address, err)
		}
	}
	fmt.Printf("%#v\n", flags.hubFlags)
	return nil
	prompt := "Where should the token be bridged as an ERC-20?"
	if cancel, err := promptChain(prompt, network, flags.hubFlags.chainFlags.CChain, flags.hubFlags.chainFlags.SubnetName, &flags.spokeFlags); err != nil {
		return err
	} else if cancel {
		return nil
	}
	return nil
}

func filterSubnetsByNetwork(network models.Network, subnetNames []string) ([]string, error) {
	filtered := []string{}
	for _, subnetName := range subnetNames {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return nil, err
		}
		if sc.Networks[network.Name()].BlockchainID != ids.Empty {
			filtered = append(filtered, subnetName)
		}
	}
	return filtered, nil
}

func promptChain(
	prompt string,
	network models.Network,
	avoidCChain bool,
	avoidSubnet string,
	chainFlags *ChainFlags,
) (bool, error) {
	subnetNames, err := app.GetSubnetNames()
	if err != nil {
		return false, err
	}
	subnetNames, err = filterSubnetsByNetwork(network, subnetNames)
	if err != nil {
		return false, err
	}
	subnetNames = utils.RemoveFromSlice(subnetNames, avoidSubnet)
	cChainOption := "C-Chain"
	notListedOption := "My blockchain isn't listed"
	subnetOptions := []string{}
	if !avoidCChain {
		subnetOptions = append(subnetOptions, cChainOption)
	}
	subnetOptions = append(subnetOptions, utils.Map(subnetNames, func(s string) string { return "Subnet " + s })...)
	subnetOptions = append(subnetOptions, notListedOption)
	subnetOption, err := app.Prompt.CaptureListWithSize(
		prompt,
		subnetOptions,
		11,
	)
	if err != nil {
		return false, err
	}
	if subnetOption == notListedOption {
		ux.Logger.PrintToUser("Please import the subnet first, using the `avalanche subnet import` command suite")
		return true, nil
	}
	if subnetOption == cChainOption {
		chainFlags.CChain = true
	} else {
		chainFlags.SubnetName = strings.TrimPrefix(subnetOption, "Subnet ")
	}
	return false, nil
}

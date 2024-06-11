// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/bridge"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type ChainFlags struct {
	SubnetName string
	CChain     bool
}

type HubFlags struct {
	chainFlags   ChainFlags
	hubAddress   string
	native       bool
	erc20Address string
}

type DeployFlags struct {
	Network    networkoptions.NetworkFlags
	hubFlags   HubFlags
	spokeFlags ChainFlags
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
	cmd.Flags().BoolVar(&deployFlags.hubFlags.chainFlags.CChain, "c-chain-hub", false, "use C-Chain as the Bridge Hub's Chain")
	cmd.Flags().BoolVar(&deployFlags.hubFlags.native, "deploy-native-hub", false, "deploy a Bridge Hub for the Chain's Native Token")
	cmd.Flags().StringVar(&deployFlags.hubFlags.erc20Address, "deploy-erc20-hub", "", "deploy a Bridge Hub for the Chain's ERC20 Token")
	cmd.Flags().StringVar(&deployFlags.hubFlags.hubAddress, "use-hub", "", "use the given Bridge Hub Address")
	cmd.Flags().BoolVar(&deployFlags.spokeFlags.CChain, "c-chain-spoke", false, "use C-Chain as the Bridge Spoke's Chain")
	cmd.Flags().StringVar(&deployFlags.spokeFlags.SubnetName, "spoke-subnet", "", "use the given CLI subnet as the Bridge Spoke's Chain")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(_ []string, flags DeployFlags) error {
	if !bridge.FoundryIsInstalled() {
		return bridge.InstallFoundry()
	}
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

	// flags exclusiveness
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.hubFlags.chainFlags.SubnetName != "", flags.hubFlags.chainFlags.CChain}) {
		return fmt.Errorf("--hub-subnet and --c-chain-hub are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		flags.hubFlags.hubAddress != "",
		flags.hubFlags.erc20Address != "",
		flags.hubFlags.native,
	}) {
		return fmt.Errorf("--deploy-native-hub, --deploy-erc20-hub, and --use-hub are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.spokeFlags.SubnetName != "", flags.spokeFlags.CChain}) {
		return fmt.Errorf("--spoke-subnet and --c-chain-spoke are mutually exclusive flags")
	}

	// Hub Chain Prompts
	if flags.hubFlags.chainFlags.SubnetName == "" && !flags.hubFlags.chainFlags.CChain {
		prompt := "Where is the Token origin?"
		if cancel, err := promptChain(prompt, network, false, "", &flags.hubFlags.chainFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	// Hub Chain Validations
	if flags.hubFlags.chainFlags.SubnetName != "" {
		if err := validateSubnet(network, flags.hubFlags.chainFlags.SubnetName); err != nil {
			return err
		}
	}

	// Hub Contract Prompts
	if flags.hubFlags.hubAddress == "" && flags.hubFlags.erc20Address == "" && !flags.hubFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.hubFlags.chainFlags.SubnetName,
			flags.hubFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
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
				nativeOption := "The native token " + nativeTokenSymbol
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

	// Hub Contract Validations
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

	// Spoke Chain Prompts
	if !flags.spokeFlags.CChain && flags.spokeFlags.SubnetName == "" {
		prompt := "Where should the token be bridged as an ERC-20?"
		if cancel, err := promptChain(prompt, network, flags.hubFlags.chainFlags.CChain, flags.hubFlags.chainFlags.SubnetName, &flags.spokeFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	// Spoke Chain Validations
	if flags.spokeFlags.SubnetName != "" {
		if err := validateSubnet(network, flags.spokeFlags.SubnetName); err != nil {
			return err
		}
		if flags.spokeFlags.SubnetName == flags.hubFlags.chainFlags.SubnetName {
			return fmt.Errorf("trying to make a bridge were hub and spoke are on the same subnet")
		}
	}
	if flags.spokeFlags.CChain && flags.hubFlags.chainFlags.CChain {
		return fmt.Errorf("trying to make a bridge were hub and spoke are on the same subnet")
	}

	// Setup Contracts
	ux.Logger.PrintToUser("Downloading Bridge Contracts")
	if err := bridge.DownloadRepo(app); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Compiling Bridge")
	if err := bridge.BuildContracts(app); err != nil {
		return err
	}
	ux.Logger.PrintToUser("")

	// Hub Deploy
	bridgeSrcDir := utils.ExpandHome("~/Workspace/projects/teleporter-token-bridge/")
	var (
		hubAddress        common.Address
		tokenSymbol       string
		tokenName         string
		tokenDecimals     uint8
		erc20TokenAddress common.Address
	)
	// TODO: need registry address, manager address, private key for the hub chain (academy for fuji)
	hubEndpoint, _, hubBlockchainID, _, hubRegistryAddress, hubKey, err := GetSubnetParams(
		network,
		flags.hubFlags.chainFlags.SubnetName,
		flags.hubFlags.chainFlags.CChain,
	)
	if err != nil {
		return err
	}
	if flags.hubFlags.hubAddress != "" {
		hubAddress = common.HexToAddress(flags.hubFlags.hubAddress)
		erc20TokenAddress, err = bridge.GetHubERC20Address(bridgeSrcDir, hubEndpoint, hubAddress)
		if err != nil {
			return err
		}
		tokenSymbol, tokenName, tokenDecimals, err = bridge.GetTokenParams(
			hubEndpoint,
			erc20TokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
	}
	if flags.hubFlags.erc20Address != "" {
		erc20TokenAddress = common.HexToAddress(flags.hubFlags.erc20Address)
		tokenSymbol, tokenName, tokenDecimals, err = bridge.GetTokenParams(
			hubEndpoint,
			erc20TokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		hubAddress, err = bridge.DeployERC20Hub(
			bridgeSrcDir,
			hubEndpoint,
			hubKey.PrivKeyHex(),
			common.HexToAddress(hubRegistryAddress),
			common.HexToAddress(hubKey.C()),
			erc20TokenAddress,
			tokenDecimals,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Hub Deployed to %s", hubEndpoint)
		ux.Logger.PrintToUser("Hub Address: %s", hubAddress)
		ux.Logger.PrintToUser("")
	}
	if flags.hubFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.hubFlags.chainFlags.SubnetName,
			flags.hubFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		wrappedNativeTokenAddress, err := bridge.DeployWrappedNativeToken(
			bridgeSrcDir,
			hubEndpoint,
			hubKey.PrivKeyHex(),
			nativeTokenSymbol,
		)
		if err != nil {
			return err
		}
		tokenSymbol, tokenName, tokenDecimals, err = bridge.GetTokenParams(
			hubEndpoint,
			wrappedNativeTokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Wrapped Native Token Deployed to %s", hubEndpoint)
		ux.Logger.PrintToUser("%s Address: %s", tokenSymbol, wrappedNativeTokenAddress)
		ux.Logger.PrintToUser("")
		hubAddress, err = bridge.DeployNativeHub(
			bridgeSrcDir,
			hubEndpoint,
			hubKey.PrivKeyHex(),
			common.HexToAddress(hubRegistryAddress),
			common.HexToAddress(hubKey.C()),
			wrappedNativeTokenAddress,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Hub Deployed to %s", hubEndpoint)
		ux.Logger.PrintToUser("Hub Address: %s", hubAddress)
		ux.Logger.PrintToUser("")
	}

	// Spoke Deploy
	spokeEndpoint, _, _, _, spokeRegistryAddress, spokeKey, err := GetSubnetParams(
		network,
		flags.spokeFlags.SubnetName,
		flags.spokeFlags.CChain,
	)
	if err != nil {
		return err
	}

	spokeAddress, err := bridge.DeployERC20Spoke(
		bridgeSrcDir,
		spokeEndpoint,
		spokeKey.PrivKeyHex(),
		common.HexToAddress(spokeRegistryAddress),
		common.HexToAddress(spokeKey.C()),
		hubBlockchainID,
		hubAddress,
		tokenName,
		tokenSymbol,
		tokenDecimals,
	)
	if err != nil {
		return err
	}

	if err := bridge.RegisterERC20Spoke(
		bridgeSrcDir,
		spokeEndpoint,
		spokeKey.PrivKeyHex(),
		spokeAddress,
	); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Spoke Deployed to %s", spokeEndpoint)
	ux.Logger.PrintToUser("Spoke Address: %s", spokeAddress)

	return nil
}

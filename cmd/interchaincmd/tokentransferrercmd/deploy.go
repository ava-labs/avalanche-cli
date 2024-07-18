// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferrercmd

import (
	_ "embed"
	"fmt"
	"math/big"
	"time"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/ictt"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type HomeFlags struct {
	chainFlags   contract.ChainFlags
	homeAddress  string
	native       bool
	erc20Address string
}

type RemoteFlags struct {
	chainFlags contract.ChainFlags
	native     bool
}

type DeployFlags struct {
	Network     networkoptions.NetworkFlags
	homeFlags   HomeFlags
	remoteFlags RemoteFlags
	version     string
}

var (
	deploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployFlags DeployFlags
)

// avalanche interchain tokenTransferrer deploy
func NewDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a Token Transferrer into a given Network and Subnets",
		Long:  "Deploys a Token Transferrer into a given Network and Subnets",
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	contract.AddChainFlagsToCmd(
		cmd,
		&deployFlags.homeFlags.chainFlags,
		"set the Transferrer's Home Chain",
		"home-subnet",
		"c-chain-home",
	)
	contract.AddChainFlagsToCmd(
		cmd,
		&deployFlags.remoteFlags.chainFlags,
		"set the Transferrer's Remote Chain",
		"remote-subnet",
		"c-chain-remote",
	)
	cmd.Flags().BoolVar(&deployFlags.homeFlags.native, "deploy-native-home", false, "deploy a Transferrer Home for the Chain's Native Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.erc20Address, "deploy-erc20-home", "", "deploy a Transferrer Home for the given Chain's ERC20 Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.homeAddress, "use-home", "", "use the given Transferrer's Home Address")
	cmd.Flags().StringVar(&deployFlags.version, "version", "", "tag/branch/commit of Avalanche InterChain Token Transfer to be used (defaults to main branch)")
	cmd.Flags().BoolVar(&deployFlags.remoteFlags.native, "deploy-native-remote", false, "deploy a Transferrer Remote for the Chain's Native Token")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(_ []string, flags DeployFlags) error {
	if !ictt.FoundryIsInstalled() {
		if err := ictt.InstallFoundry(); err != nil {
			return err
		}
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Transferrer?",
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
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.homeFlags.chainFlags.SubnetName != "", flags.homeFlags.chainFlags.CChain}) {
		return fmt.Errorf("--home-subnet and --c-chain-home are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		flags.homeFlags.homeAddress != "",
		flags.homeFlags.erc20Address != "",
		flags.homeFlags.native,
	}) {
		return fmt.Errorf("--deploy-native-home, --deploy-erc20-home, and --use-home are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.remoteFlags.chainFlags.SubnetName != "", flags.remoteFlags.chainFlags.CChain}) {
		return fmt.Errorf("--remote-subnet and --c-chain-remote are mutually exclusive flags")
	}

	// Home Chain Prompts
	if flags.homeFlags.chainFlags.SubnetName == "" && !flags.homeFlags.chainFlags.CChain {
		prompt := "Where is the Token origin?"
		if cancel, err := promptChain(prompt, network, false, "", &flags.homeFlags.chainFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	// Home Chain Validations
	if flags.homeFlags.chainFlags.SubnetName != "" {
		if err := validateSubnet(network, flags.homeFlags.chainFlags.SubnetName); err != nil {
			return err
		}
	}

	// Home Contract Prompts
	if flags.homeFlags.homeAddress == "" && flags.homeFlags.erc20Address == "" && !flags.homeFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.homeFlags.chainFlags.SubnetName,
			flags.homeFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		prompt := "What kind of token do you want to be able to transfer?"
		popularOption := "A popular token (e.g. AVAX, USDC, WAVAX, ...) (recommended)"
		homeDeployedOption := "A token that already has a Home deployed (recommended)"
		deployNewHomeOption := "Deploy a new Home for the token"
		explainOption := "Explain the difference"
		goBackOption := "Go Back"
		homeChain := "C-Chain"
		if !flags.homeFlags.chainFlags.CChain {
			homeChain = flags.homeFlags.chainFlags.SubnetName
		}
		popularTokensInfo, err := GetPopularTokensInfo(network, homeChain)
		if err != nil {
			return err
		}
		popularTokensDesc := utils.Map(
			popularTokensInfo,
			func(i PopularTokenInfo) string {
				return i.Desc()
			},
		)
		options := []string{popularOption, homeDeployedOption, deployNewHomeOption, explainOption}
		if len(popularTokensDesc) == 0 {
			options = []string{homeDeployedOption, deployNewHomeOption, explainOption}
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
			case homeDeployedOption:
				addr, err := app.Prompt.CaptureAddress(
					"Enter the address of the Home",
				)
				if err != nil {
					return err
				}
				flags.homeFlags.homeAddress = addr.Hex()
			case deployNewHomeOption:
				nativeOption := "The native token " + nativeTokenSymbol
				erc20Option := "An ERC-20 token"
				options := []string{nativeOption, erc20Option}
				option, err := app.Prompt.CaptureList(
					"What kind of token do you want to deploy the Home for?",
					options,
				)
				if err != nil {
					return err
				}
				switch option {
				case nativeOption:
					flags.homeFlags.native = true
				case erc20Option:
					erc20TokenAddr, err := app.Prompt.CaptureAddress(
						"Enter the address of the ERC-20 Token",
					)
					if err != nil {
						return err
					}
					flags.homeFlags.erc20Address = erc20TokenAddr.Hex()
					if p := utils.Find(popularTokensInfo, func(p PopularTokenInfo) bool { return p.TokenContractAddress == erc20TokenAddr.Hex() }); p != nil {
						ux.Logger.PrintToUser("There already is a Token Home for %s deployed on %s.", p.TokenName, homeChain)
						ux.Logger.PrintToUser("")
						ux.Logger.PrintToUser("Home Address: %s", p.TransferrerHomeAddress)
						deployANewHupOption := "Yes, use the existing Home (recommended)"
						useTheExistingHomeOption := "No, deploy my own Home"
						options := []string{deployANewHupOption, useTheExistingHomeOption, explainOption}
						option, err := app.Prompt.CaptureList(
							"Do you want to use the existing Home?",
							options,
						)
						if err != nil {
							return err
						}
						switch option {
						case useTheExistingHomeOption:
							flags.homeFlags.homeAddress = p.TransferrerHomeAddress
							flags.homeFlags.erc20Address = ""
						case deployANewHupOption:
						case explainOption:
							ux.Logger.PrintToUser("There is already a Transferrer Home deployed for the popular token %s on %s.",
								p.TokenName,
								homeChain,
							)
							ux.Logger.PrintToUser("Connect to that Home to participate in standard cross chain transfers")
							ux.Logger.PrintToUser("for the token, including transfers to any of the registered Remote subnets.")
							ux.Logger.PrintToUser("Deploy a new Home if wanting to have isolated cross chain transfers for")
							ux.Logger.PrintToUser("your application, or if wanting to provide a new Transferrer alternative")
							ux.Logger.PrintToUser("for the token.")
						}
					}
				}
			case explainOption:
				ux.Logger.PrintToUser("An Avalanche InterChain Token Transferrer consists of one Home and at least one but possibly many Remotes.")
				ux.Logger.PrintToUser("The Home manages the asset to be shared to Remote instances. It lives on the Subnet")
				ux.Logger.PrintToUser("where the asset exists")
				ux.Logger.PrintToUser("The Remotes live on the other Subnets that want to import the asset managed by the Home.")
				ux.Logger.PrintToUser("")
				if len(popularTokensDesc) != 0 {
					ux.Logger.PrintToUser("A popular token of a subnet is assumed to already have a Home Deployed. In this case")
					ux.Logger.PrintToUser("the Home parameters will be automatically obtained, and a new Remote will be created on")
					ux.Logger.PrintToUser("the other Subnet, to access the popular token.")
				}
				ux.Logger.PrintToUser("For a token that already has a Home deployed, the Home parameters will be prompted,")
				ux.Logger.PrintToUser("and a new Remote will be created on the other Subnet to access that token.")
				ux.Logger.PrintToUser("If deploying a new Home for the token, the token parameters will be prompted,")
				ux.Logger.PrintToUser("and both a new Home will be created on the token Subnet, and a new Remote will be created")
				ux.Logger.PrintToUser("on the other Subnet to access that token.")
				continue
			}
			break
		}
	}

	// Home Contract Validations
	if flags.homeFlags.homeAddress != "" {
		if err := prompts.ValidateAddress(flags.homeFlags.homeAddress); err != nil {
			return fmt.Errorf("failure validating %s: %w", flags.homeFlags.homeAddress, err)
		}
	}
	if flags.homeFlags.erc20Address != "" {
		if err := prompts.ValidateAddress(flags.homeFlags.erc20Address); err != nil {
			return fmt.Errorf("failure validating %s: %w", flags.homeFlags.erc20Address, err)
		}
	}

	// Remote Chain Prompts
	if !flags.remoteFlags.chainFlags.CChain && flags.remoteFlags.chainFlags.SubnetName == "" {
		prompt := "Where should the token be available as an ERC-20?"
		if cancel, err := promptChain(prompt, network, flags.homeFlags.chainFlags.CChain, flags.homeFlags.chainFlags.SubnetName, &flags.remoteFlags.chainFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	// Remote Chain Validations
	if flags.remoteFlags.chainFlags.SubnetName != "" {
		if err := validateSubnet(network, flags.remoteFlags.chainFlags.SubnetName); err != nil {
			return err
		}
		if flags.remoteFlags.chainFlags.SubnetName == flags.homeFlags.chainFlags.SubnetName {
			return fmt.Errorf("trying to make an Transferrer were home and remote are on the same subnet")
		}
	}
	if flags.remoteFlags.chainFlags.CChain && flags.homeFlags.chainFlags.CChain {
		return fmt.Errorf("trying to make an Transferrer were home and remote are on the same subnet")
	}

	// Setup Contracts
	ux.Logger.PrintToUser("Downloading Avalanche InterChain Token Transfer Contracts")
	if err := ictt.DownloadRepo(app, flags.version); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Compiling Avalanche InterChain Token Transfer")
	if err := ictt.BuildContracts(app); err != nil {
		return err
	}
	ux.Logger.PrintToUser("")

	// Home Deploy
	icttSrcDir, err := ictt.RepoDir(app)
	if err != nil {
		return err
	}
	var (
		homeAddress   common.Address
		tokenSymbol   string
		tokenName     string
		tokenDecimals uint8
		tokenAddress  common.Address
	)
	// TODO: need registry address, manager address, private key for the home chain (academy for fuji)
	homeEndpoint, _, _, homeBlockchainID, _, homeRegistryAddress, homeKey, err := teleporter.GetSubnetParams(
		app,
		network,
		flags.homeFlags.chainFlags.SubnetName,
		flags.homeFlags.chainFlags.CChain,
	)
	if err != nil {
		return err
	}
	if flags.homeFlags.homeAddress != "" {
		homeAddress = common.HexToAddress(flags.homeFlags.homeAddress)
		endpointKind, err := ictt.GetEndpointKind(homeEndpoint, homeAddress)
		if err != nil {
			return err
		}
		switch endpointKind {
		case ictt.ERC20TokenHome:
			tokenAddress, err = ictt.ERC20TokenHomeGetTokenAddress(homeEndpoint, homeAddress)
			if err != nil {
				return err
			}
		case ictt.NativeTokenHome:
			tokenAddress, err = ictt.NativeTokenHomeGetTokenAddress(homeEndpoint, homeAddress)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported ictt endpoint kind %d", endpointKind)
		}
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeEndpoint,
			tokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
	}
	if flags.homeFlags.erc20Address != "" {
		tokenAddress = common.HexToAddress(flags.homeFlags.erc20Address)
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeEndpoint,
			tokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		homeAddress, err = ictt.DeployERC20Home(
			icttSrcDir,
			homeEndpoint,
			homeKey.PrivKeyHex(),
			common.HexToAddress(homeRegistryAddress),
			common.HexToAddress(homeKey.C()),
			tokenAddress,
			tokenDecimals,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Home Deployed to %s", homeEndpoint)
		ux.Logger.PrintToUser("Home Address: %s", homeAddress)
		ux.Logger.PrintToUser("")
	}
	if flags.homeFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.homeFlags.chainFlags.SubnetName,
			flags.homeFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		wrappedNativeTokenAddress, err := ictt.DeployWrappedNativeToken(
			icttSrcDir,
			homeEndpoint,
			homeKey.PrivKeyHex(),
			nativeTokenSymbol,
		)
		if err != nil {
			return err
		}
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeEndpoint,
			wrappedNativeTokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Wrapped Native Token Deployed to %s", homeEndpoint)
		ux.Logger.PrintToUser("%s Address: %s", tokenSymbol, wrappedNativeTokenAddress)
		ux.Logger.PrintToUser("")
		homeAddress, err = ictt.DeployNativeHome(
			icttSrcDir,
			homeEndpoint,
			homeKey.PrivKeyHex(),
			common.HexToAddress(homeRegistryAddress),
			common.HexToAddress(homeKey.C()),
			wrappedNativeTokenAddress,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Home Deployed to %s", homeEndpoint)
		ux.Logger.PrintToUser("Home Address: %s", homeAddress)
		ux.Logger.PrintToUser("")
	}

	// Remote Deploy
	remoteEndpoint, remoteBlockchainName, _, remoteBlockchainID, _, remoteRegistryAddress, remoteKey, err := teleporter.GetSubnetParams(
		app,
		network,
		flags.remoteFlags.chainFlags.SubnetName,
		flags.remoteFlags.chainFlags.CChain,
	)
	if err != nil {
		return err
	}

	var (
		remoteAddress common.Address
		remoteSupply  *big.Int
	)

	if !flags.remoteFlags.native {
		remoteAddress, err = ictt.DeployERC20Remote(
			icttSrcDir,
			remoteEndpoint,
			remoteKey.PrivKeyHex(),
			common.HexToAddress(remoteRegistryAddress),
			common.HexToAddress(remoteKey.C()),
			homeBlockchainID,
			homeAddress,
			tokenName,
			tokenSymbol,
			tokenDecimals,
		)
		if err != nil {
			return err
		}
	} else {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.remoteFlags.chainFlags.SubnetName,
			flags.remoteFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		remoteSupply, err = contract.GetEVMSubnetGenesisSupply(
			app,
			network,
			flags.remoteFlags.chainFlags.SubnetName,
			flags.remoteFlags.chainFlags.CChain,
			"",
		)
		if err != nil {
			return err
		}
		remoteAddress, err = ictt.DeployNativeRemote(
			icttSrcDir,
			remoteEndpoint,
			remoteKey.PrivKeyHex(),
			common.HexToAddress(remoteRegistryAddress),
			common.HexToAddress(remoteKey.C()),
			homeBlockchainID,
			homeAddress,
			tokenDecimals,
			nativeTokenSymbol,
			remoteSupply,
			big.NewInt(0),
		)
		if err != nil {
			return err
		}
	}

	if err := ictt.RegisterERC20Remote(
		remoteEndpoint,
		remoteKey.PrivKeyHex(),
		remoteAddress,
	); err != nil {
		return err
	}

	checkInterval := 100 * time.Millisecond
	checkTimeout := 10 * time.Second
	t0 := time.Now()
	for {
		registeredRemote, err := ictt.TokenHomeGetRegisteredRemote(
			homeEndpoint,
			homeAddress,
			remoteBlockchainID,
			remoteAddress,
		)
		if err != nil {
			return err
		}
		if registeredRemote.Registered {
			break
		}
		elapsed := time.Since(t0)
		if elapsed > checkTimeout {
			return fmt.Errorf("timeout waiting for remote endpoint registration")
		}
		time.Sleep(checkInterval)
	}

	if flags.remoteFlags.native {
		err = ictt.TokenHomeAddCollateral(
			homeEndpoint,
			homeAddress,
			homeKey.PrivKeyHex(),
			remoteBlockchainID,
			remoteAddress,
			remoteSupply,
		)
		if err != nil {
			return err
		}

		registeredRemote, err := ictt.TokenHomeGetRegisteredRemote(
			homeEndpoint,
			homeAddress,
			remoteBlockchainID,
			remoteAddress,
		)
		if err != nil {
			return err
		}
		if registeredRemote.CollateralNeeded.Cmp(big.NewInt(0)) != 0 {
			return fmt.Errorf("failure setting collateral in home endpoint: remaining collateral=%d", registeredRemote.CollateralNeeded)
		}

		minterAdminFound, managedMinterAdmin, _, minterAdminAddress, minterAdminPrivKey, err := contract.GetEVMSubnetGenesisNativeMinterAdmin(
			app,
			network,
			flags.remoteFlags.chainFlags.SubnetName,
			flags.remoteFlags.chainFlags.CChain,
			"",
		)
		if err != nil {
			return err
		}
		if !minterAdminFound {
			return fmt.Errorf("there is no native minter precompile admin on subnet %s", remoteBlockchainName)
		}
		if !managedMinterAdmin {
			return fmt.Errorf("no managed key found for native minter admin %s subnet %s", minterAdminAddress, remoteBlockchainName)
		}

		if err := ictt.EnableMinter(
			remoteEndpoint,
			minterAdminPrivKey,
			remoteAddress,
		); err != nil {
			return err
		}

		err = ictt.Send(
			homeEndpoint,
			homeAddress,
			homeKey.PrivKeyHex(),
			remoteBlockchainID,
			remoteAddress,
			common.HexToAddress(homeKey.C()),
			big.NewInt(1),
		)
		if err != nil {
			return err
		}
		t0 := time.Now()
		for {
			isCollateralized, err := ictt.TokenRemoteIsCollateralized(
				remoteEndpoint,
				remoteAddress,
			)
			if err != nil {
				return err
			}
			if isCollateralized {
				break
			}
			elapsed := time.Since(t0)
			if elapsed > checkTimeout {
				return fmt.Errorf("timeout waiting for remote endpoint collateralization")
			}
			time.Sleep(checkInterval)
		}
	}

	ux.Logger.PrintToUser("Remote Deployed to %s", remoteEndpoint)
	ux.Logger.PrintToUser("Remote Address: %s", remoteAddress)

	return nil
}

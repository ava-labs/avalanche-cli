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
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/ictt"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/precompiles"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/spf13/cobra"
)

type HomeFlags struct {
	chainFlags      contract.ChainSpec
	homeAddress     string
	native          bool
	erc20Address    string
	privateKeyFlags contract.PrivateKeyFlags
	RPCEndpoint     string
}

type RemoteFlags struct {
	chainFlags        contract.ChainSpec
	native            bool
	removeMinterAdmin bool
	privateKeyFlags   contract.PrivateKeyFlags
	RPCEndpoint       string
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
	contract.AddChainSpecToCmd(
		cmd,
		&deployFlags.homeFlags.chainFlags,
		"set the Transferrer's Home Chain",
		"home-subnet",
		"c-chain-home",
		"",
		false,
	)
	contract.AddChainSpecToCmd(
		cmd,
		&deployFlags.remoteFlags.chainFlags,
		"set the Transferrer's Remote Chain",
		"remote-subnet",
		"c-chain-remote",
		"",
		false,
	)
	cmd.Flags().BoolVar(&deployFlags.homeFlags.native, "deploy-native-home", false, "deploy a Transferrer Home for the Chain's Native Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.erc20Address, "deploy-erc20-home", "", "deploy a Transferrer Home for the given Chain's ERC20 Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.homeAddress, "use-home", "", "use the given Transferrer's Home Address")
	cmd.Flags().StringVar(&deployFlags.version, "version", "", "tag/branch/commit of Avalanche InterChain Token Transfer to be used (defaults to main branch)")
	cmd.Flags().BoolVar(&deployFlags.remoteFlags.native, "deploy-native-remote", false, "deploy a Transferrer Remote for the Chain's Native Token")
	cmd.Flags().BoolVar(&deployFlags.remoteFlags.removeMinterAdmin, "remove-minter-admin", true, "remove the native minter precompile admin found on remote blockchain genesis")
	contract.AddPrivateKeyFlagsToCmd(
		cmd,
		&deployFlags.homeFlags.privateKeyFlags,
		"to deploy Transferrer Home",
		"home-private-key",
		"home-key",
		"home-genesis-key",
	)
	contract.AddPrivateKeyFlagsToCmd(
		cmd,
		&deployFlags.remoteFlags.privateKeyFlags,
		"to deploy Transferrer Remote",
		"remote-private-key",
		"remote-key",
		"remote-genesis-key",
	)
	cmd.Flags().StringVar(&deployFlags.homeFlags.RPCEndpoint, "home-rpc", "", "use the given RPC URL to connect to the home blockchain")
	cmd.Flags().StringVar(&deployFlags.remoteFlags.RPCEndpoint, "remote-rpc", "", "use the given RPC URL to connect to the remote blockchain")
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
	if !contract.MutuallyExclusiveChainSpecFields(flags.homeFlags.chainFlags) {
		return fmt.Errorf("--home-subnet and --c-chain-home are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		flags.homeFlags.homeAddress != "",
		flags.homeFlags.erc20Address != "",
		flags.homeFlags.native,
	}) {
		return fmt.Errorf("--deploy-native-home, --deploy-erc20-home, and --use-home are mutually exclusive flags")
	}
	if !contract.MutuallyExclusiveChainSpecFields(flags.remoteFlags.chainFlags) {
		return fmt.Errorf("--remote-subnet and --c-chain-remote are mutually exclusive flags")
	}

	// Home Chain Prompts
	if !contract.DefinedChainSpec(flags.homeFlags.chainFlags) {
		prompt := "Where is the Token origin?"
		if cancel, err := contract.PromptChain(app, network, prompt, false, "", false, &flags.homeFlags.chainFlags); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}
	homeRPCEndpoint := flags.homeFlags.RPCEndpoint
	if homeRPCEndpoint == "" {
		homeRPCEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, flags.homeFlags.chainFlags, true, false)
		if err != nil {
			return err
		}
	}

	// Home Chain Validations
	if flags.homeFlags.chainFlags.BlockchainName != "" {
		if err := validateSubnet(network, flags.homeFlags.chainFlags.BlockchainName); err != nil {
			return err
		}
	}

	// Home Contract Prompts
	if flags.homeFlags.homeAddress == "" && flags.homeFlags.erc20Address == "" && !flags.homeFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.homeFlags.chainFlags.BlockchainName,
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
			homeChain = flags.homeFlags.chainFlags.BlockchainName
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

	var (
		homeKey        string
		homeKeyAddress string
	)
	if flags.homeFlags.homeAddress == "" {
		genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
			app,
			network,
			flags.homeFlags.chainFlags,
		)
		if err != nil {
			return err
		}
		homeKey, err := contract.GetPrivateKeyFromFlags(
			app,
			flags.homeFlags.privateKeyFlags,
			genesisPrivateKey,
			"--home-private-key, --home-key and --home-genesis-key are mutually exclusive flags",
		)
		if err != nil {
			return err
		}
		if homeKey == "" {
			homeKey, err = prompts.PromptPrivateKey(
				app.Prompt,
				"pay for home deploy fees",
				app.GetKeyDir(),
				app.GetKey,
				genesisAddress,
				genesisPrivateKey,
			)
			if err != nil {
				return err
			}
		}
		pk, err := crypto.HexToECDSA(homeKey)
		if err != nil {
			return err
		}
		homeKeyAddress = crypto.PubkeyToAddress(pk.PublicKey).Hex()
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
	if !contract.DefinedChainSpec(flags.remoteFlags.chainFlags) {
		prompt := "Where should the token be available as an ERC-20?"
		if flags.remoteFlags.native {
			prompt = "Where should the token be available as a Native Token?"
		}
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			flags.homeFlags.chainFlags.CChain,
			flags.homeFlags.chainFlags.BlockchainName,
			false,
			&flags.remoteFlags.chainFlags,
		); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}
	remoteRPCEndpoint := flags.remoteFlags.RPCEndpoint
	if remoteRPCEndpoint == "" {
		remoteRPCEndpoint, _, err = contract.GetBlockchainEndpoints(app, network, flags.remoteFlags.chainFlags, true, false)
		if err != nil {
			return err
		}
	}

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		flags.remoteFlags.chainFlags,
	)
	if err != nil {
		return err
	}
	remoteKey, err := contract.GetPrivateKeyFromFlags(
		app,
		flags.remoteFlags.privateKeyFlags,
		genesisPrivateKey,
		"--remote-private-key, --remote-key and --remote-genesis-key are mutually exclusive flags",
	)
	if err != nil {
		return err
	}
	if remoteKey == "" {
		remoteKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"pay for home deploy fees",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	pk, err := crypto.HexToECDSA(remoteKey)
	if err != nil {
		return err
	}
	remoteKeyAddress := crypto.PubkeyToAddress(pk.PublicKey).Hex()

	// Remote Chain Validations
	if flags.remoteFlags.chainFlags.BlockchainName != "" {
		if err := validateSubnet(network, flags.remoteFlags.chainFlags.BlockchainName); err != nil {
			return err
		}
		if flags.remoteFlags.chainFlags.BlockchainName == flags.homeFlags.chainFlags.BlockchainName {
			return fmt.Errorf("trying to make an Transferrer were home and remote are on the same subnet")
		}
	}
	if flags.remoteFlags.chainFlags.CChain && flags.homeFlags.chainFlags.CChain {
		return fmt.Errorf("trying to make an Transferrer were home and remote are on the same subnet")
	}

	// Setup Contracts
	ux.Logger.PrintToUser("Downloading Avalanche InterChain Token Transfer Contracts")
	version := constants.ICTTVersion
	if flags.version != "" {
		version = flags.version
	}
	if err := ictt.DownloadRepo(app, version); err != nil {
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
	homeBlockchainID, err := contract.GetBlockchainID(app, network, flags.homeFlags.chainFlags)
	if err != nil {
		return err
	}
	homeRegistryAddress, _, err := contract.GetICMInfo(app, network, flags.homeFlags.chainFlags, true, false, true)
	if err != nil {
		return err
	}
	if flags.homeFlags.homeAddress != "" {
		homeAddress = common.HexToAddress(flags.homeFlags.homeAddress)
		endpointKind, err := ictt.GetEndpointKind(homeRPCEndpoint, homeAddress)
		if err != nil {
			return err
		}
		switch endpointKind {
		case ictt.ERC20TokenHome:
			tokenAddress, err = ictt.ERC20TokenHomeGetTokenAddress(homeRPCEndpoint, homeAddress)
			if err != nil {
				return err
			}
		case ictt.NativeTokenHome:
			tokenAddress, err = ictt.NativeTokenHomeGetTokenAddress(homeRPCEndpoint, homeAddress)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported ictt endpoint kind %d", endpointKind)
		}
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeRPCEndpoint,
			tokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
	}
	if flags.homeFlags.erc20Address != "" {
		tokenAddress = common.HexToAddress(flags.homeFlags.erc20Address)
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeRPCEndpoint,
			tokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		homeAddress, err = ictt.DeployERC20Home(
			icttSrcDir,
			homeRPCEndpoint,
			homeKey,
			common.HexToAddress(homeRegistryAddress),
			common.HexToAddress(homeKeyAddress),
			tokenAddress,
			tokenDecimals,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Home Deployed to %s", homeRPCEndpoint)
		ux.Logger.PrintToUser("Home Address: %s", homeAddress)
		ux.Logger.PrintToUser("")
	}
	if flags.homeFlags.native {
		nativeTokenSymbol, err := getNativeTokenSymbol(
			flags.homeFlags.chainFlags.BlockchainName,
			flags.homeFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		wrappedNativeTokenAddress, err := ictt.DeployWrappedNativeToken(
			icttSrcDir,
			homeRPCEndpoint,
			homeKey,
			nativeTokenSymbol,
		)
		if err != nil {
			return err
		}
		tokenSymbol, tokenName, tokenDecimals, err = ictt.GetTokenParams(
			homeRPCEndpoint,
			wrappedNativeTokenAddress.Hex(),
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Wrapped Native Token Deployed to %s", homeRPCEndpoint)
		ux.Logger.PrintToUser("%s Address: %s", tokenSymbol, wrappedNativeTokenAddress)
		ux.Logger.PrintToUser("")
		homeAddress, err = ictt.DeployNativeHome(
			icttSrcDir,
			homeRPCEndpoint,
			homeKey,
			common.HexToAddress(homeRegistryAddress),
			common.HexToAddress(homeKeyAddress),
			wrappedNativeTokenAddress,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Home Deployed to %s", homeRPCEndpoint)
		ux.Logger.PrintToUser("Home Address: %s", homeAddress)
		ux.Logger.PrintToUser("")
	}

	// Remote Deploy
	remoteBlockchainDesc, err := contract.GetBlockchainDesc(flags.remoteFlags.chainFlags)
	if err != nil {
		return err
	}
	remoteBlockchainID, err := contract.GetBlockchainID(app, network, flags.remoteFlags.chainFlags)
	if err != nil {
		return err
	}
	remoteRegistryAddress, _, err := contract.GetICMInfo(app, network, flags.remoteFlags.chainFlags, true, false, true)
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
			remoteRPCEndpoint,
			remoteKey,
			common.HexToAddress(remoteRegistryAddress),
			common.HexToAddress(remoteKeyAddress),
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
			flags.remoteFlags.chainFlags.BlockchainName,
			flags.remoteFlags.chainFlags.CChain,
		)
		if err != nil {
			return err
		}
		remoteSupply, err = contract.GetEVMSubnetGenesisSupply(
			app,
			network,
			flags.remoteFlags.chainFlags,
		)
		if err != nil {
			return err
		}
		remoteAddress, err = ictt.DeployNativeRemote(
			icttSrcDir,
			remoteRPCEndpoint,
			remoteKey,
			common.HexToAddress(remoteRegistryAddress),
			common.HexToAddress(remoteKeyAddress),
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
		remoteRPCEndpoint,
		remoteKey,
		remoteAddress,
	); err != nil {
		return err
	}

	checkInterval := 100 * time.Millisecond
	checkTimeout := 10 * time.Second
	t0 := time.Now()
	for {
		registeredRemote, err := ictt.TokenHomeGetRegisteredRemote(
			homeRPCEndpoint,
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
			homeRPCEndpoint,
			homeAddress,
			homeKey,
			remoteBlockchainID,
			remoteAddress,
			remoteSupply,
		)
		if err != nil {
			return err
		}

		registeredRemote, err := ictt.TokenHomeGetRegisteredRemote(
			homeRPCEndpoint,
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
			flags.remoteFlags.chainFlags,
		)
		if err != nil {
			return err
		}
		if !minterAdminFound {
			return fmt.Errorf("there is no native minter precompile admin on %s", remoteBlockchainDesc)
		}
		if !managedMinterAdmin {
			return fmt.Errorf("no managed key found for native minter admin %s on %s", minterAdminAddress, remoteBlockchainDesc)
		}

		if err := precompiles.SetEnabled(
			remoteRPCEndpoint,
			precompiles.NativeMinterPrecompile,
			minterAdminPrivKey,
			remoteAddress,
		); err != nil {
			return err
		}

		err = ictt.Send(
			homeRPCEndpoint,
			homeAddress,
			homeKey,
			remoteBlockchainID,
			remoteAddress,
			common.HexToAddress(homeKeyAddress),
			big.NewInt(1),
		)
		if err != nil {
			return err
		}

		t0 := time.Now()
		for {
			isCollateralized, err := ictt.TokenRemoteIsCollateralized(
				remoteRPCEndpoint,
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

		if flags.remoteFlags.removeMinterAdmin {
			if err := precompiles.SetNone(
				remoteRPCEndpoint,
				precompiles.NativeMinterPrecompile,
				minterAdminPrivKey,
				common.HexToAddress(minterAdminAddress),
			); err != nil {
				return err
			}
		}
	}

	ux.Logger.PrintToUser("Remote Deployed to %s", remoteRPCEndpoint)
	ux.Logger.PrintToUser("Remote Address: %s", remoteAddress)

	return nil
}

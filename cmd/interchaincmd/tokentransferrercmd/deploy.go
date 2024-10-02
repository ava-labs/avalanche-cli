// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferrercmd

import (
	"errors"
	"fmt"
	"math/big"
	"time"

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

	_ "embed"

	"github.com/ava-labs/avalanchego/utils/logging"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
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
	Decimals          uint8
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
	deployFlags.homeFlags.chainFlags.SetFlagNames(
		"home-blockchain",
		"c-chain-home",
		"",
	)
	deployFlags.homeFlags.chainFlags.AddToCmd(cmd, "set the Transferrer's Home Chain", false)
	deployFlags.remoteFlags.chainFlags.SetFlagNames(
		"remote-blockchain",
		"c-chain-remote",
		"",
	)
	deployFlags.remoteFlags.chainFlags.AddToCmd(cmd, "set the Transferrer's Remote Chain", false)
	cmd.Flags().BoolVar(&deployFlags.homeFlags.native, "deploy-native-home", false, "deploy a Transferrer Home for the Chain's Native Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.erc20Address, "deploy-erc20-home", "", "deploy a Transferrer Home for the given Chain's ERC20 Token")
	cmd.Flags().StringVar(&deployFlags.homeFlags.homeAddress, "use-home", "", "use the given Transferrer's Home Address")
	cmd.Flags().StringVar(&deployFlags.version, "version", "", "tag/branch/commit of Avalanche InterChain Token Transfer to be used (defaults to main branch)")
	cmd.Flags().BoolVar(&deployFlags.remoteFlags.native, "deploy-native-remote", false, "deploy a Transferrer Remote for the Chain's Native Token")
	cmd.Flags().BoolVar(&deployFlags.remoteFlags.removeMinterAdmin, "remove-minter-admin", false, "remove the native minter precompile admin found on remote blockchain genesis")
	deployFlags.homeFlags.privateKeyFlags.SetFlagNames("home-private-key", "home-key", "home-genesis-key")
	deployFlags.homeFlags.privateKeyFlags.AddToCmd(cmd, "to deploy Transferrer Home")
	deployFlags.remoteFlags.privateKeyFlags.SetFlagNames("remote-private-key", "remote-key", "remote-genesis-key")
	deployFlags.remoteFlags.privateKeyFlags.AddToCmd(cmd, "to deploy Transferrer Remote")
	cmd.Flags().StringVar(&deployFlags.homeFlags.RPCEndpoint, "home-rpc", "", "use the given RPC URL to connect to the home blockchain")
	cmd.Flags().StringVar(&deployFlags.remoteFlags.RPCEndpoint, "remote-rpc", "", "use the given RPC URL to connect to the remote blockchain")
	cmd.Flags().Uint8Var(&deployFlags.remoteFlags.Decimals, "remote-token-decimals", 0, "use the given number of token decimals for the Transferrer Remote [defaults to token home's decimals (18 for a new wrapped native home token)]")
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
	if err := flags.homeFlags.chainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		flags.homeFlags.homeAddress != "",
		flags.homeFlags.erc20Address != "",
		flags.homeFlags.native,
	}) {
		return errors.New("--deploy-native-home, --deploy-erc20-home, and --use-home are mutually exclusive flags")
	}
	if err := flags.remoteFlags.chainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}

	// Home Chain Prompts
	if !flags.homeFlags.chainFlags.Defined() {
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
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Home RPC Endpoint: %s"), homeRPCEndpoint)
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
		popularOption := "A popular token (e.g. WAVAX, USDC, ...) (recommended)"
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
				p := utils.Find(popularTokensInfo, func(p PopularTokenInfo) bool { return p.Desc() == option })
				if p == nil {
					return errors.New("expected to have found a popular token from option")
				}
				flags.homeFlags.homeAddress = p.TransferrerHomeAddress
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
						useTheExistingHomeOption := "Yes, use the existing Home (recommended)"
						deployANewHupOption := "No, deploy my own Home"
						options := []string{useTheExistingHomeOption, deployANewHupOption, explainOption}
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
		homeKey, err = flags.homeFlags.privateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
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
	if !flags.remoteFlags.chainFlags.Defined() {
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
		ux.Logger.PrintToUser(logging.Yellow.Wrap("Remote RPC Endpoint: %s"), remoteRPCEndpoint)
	}

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		flags.remoteFlags.chainFlags,
	)
	if err != nil {
		return err
	}
	remoteKey, err := flags.remoteFlags.privateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
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
			return errors.New("trying to make an Transferrer were home and remote are on the same subnet")
		}
	}
	if flags.remoteFlags.chainFlags.CChain && flags.homeFlags.chainFlags.CChain {
		return errors.New("trying to make an Transferrer were home and remote are on the same subnet")
	}

	// Checkout minter availability for native remote before doing something else
	remoteBlockchainDesc, err := contract.GetBlockchainDesc(flags.remoteFlags.chainFlags)
	if err != nil {
		return err
	}
	var (
		remoteMinterManagerPrivKey, remoteMinterManagerAddress string
		remoteMinterManagerIsAdmin                             bool
	)
	if flags.remoteFlags.native {
		var remoteMinterAdminFound, remoteManagedMinterAdmin bool
		remoteMinterAdminFound, remoteManagedMinterAdmin, _, remoteMinterManagerAddress, remoteMinterManagerPrivKey, err = contract.GetEVMSubnetGenesisNativeMinterAdmin(
			app,
			network,
			flags.remoteFlags.chainFlags,
		)
		if err != nil {
			return err
		}
		if !remoteManagedMinterAdmin {
			remoteMinterAdminAddress := remoteMinterManagerAddress
			var remoteMinterManagerFound, remoteManagedMinterManager bool
			remoteMinterManagerFound, remoteManagedMinterManager, _, remoteMinterManagerAddress, remoteMinterManagerPrivKey, err = contract.GetEVMSubnetGenesisNativeMinterManager(
				app,
				network,
				flags.remoteFlags.chainFlags,
			)
			if err != nil {
				return err
			}
			if !remoteMinterManagerFound {
				return fmt.Errorf("there is no native minter precompile admin or manager on %s", remoteBlockchainDesc)
			}
			if !remoteManagedMinterManager {
				if remoteMinterAdminFound {
					ux.Logger.PrintToUser("no managed key found for native minter admin %s on %s. add a CLI key for it using 'avalanche key create --file'", remoteMinterAdminAddress, remoteBlockchainDesc)
				}
				return fmt.Errorf("no managed key found for native minter manager %s on %s. add a CLI key for it using 'avalanche key create --file'", remoteMinterManagerAddress, remoteBlockchainDesc)
			}
		} else {
			remoteMinterManagerIsAdmin = true
		}
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
	var homeAddress common.Address
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
	}
	if flags.homeFlags.erc20Address != "" {
		tokenHomeAddress := common.HexToAddress(flags.homeFlags.erc20Address)
		tokenHomeDecimals, err := ictt.GetTokenDecimals(
			homeRPCEndpoint,
			tokenHomeAddress,
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
			tokenHomeAddress,
			tokenHomeDecimals,
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
		ux.Logger.PrintToUser("Wrapped Native Token Deployed to %s", homeRPCEndpoint)
		ux.Logger.PrintToUser("%s Address: %s", nativeTokenSymbol, wrappedNativeTokenAddress)
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

	// get token home symbol, name, decimals
	endpointKind, err := ictt.GetEndpointKind(homeRPCEndpoint, homeAddress)
	if err != nil {
		return err
	}
	var tokenHomeAddress common.Address
	switch endpointKind {
	case ictt.ERC20TokenHome:
		tokenHomeAddress, err = ictt.ERC20TokenHomeGetTokenAddress(homeRPCEndpoint, homeAddress)
		if err != nil {
			return err
		}
	case ictt.NativeTokenHome:
		tokenHomeAddress, err = ictt.NativeTokenHomeGetTokenAddress(homeRPCEndpoint, homeAddress)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported ictt endpoint kind %d", endpointKind)
	}
	tokenHomeSymbol, tokenHomeName, _, err := ictt.GetTokenParams(
		homeRPCEndpoint,
		tokenHomeAddress,
	)
	if err != nil {
		return err
	}
	homeDecimals, err := ictt.TokenHomeGetDecimals(homeRPCEndpoint, homeAddress)
	if err != nil {
		return err
	}

	if !flags.remoteFlags.native {
		// we default token remote decimals to be the same as token home decimals,
		// but allow to be overridden by a user's provided flag
		remoteDecimals := homeDecimals
		if flags.remoteFlags.Decimals != 0 {
			remoteDecimals = flags.remoteFlags.Decimals
		}
		remoteAddress, err = ictt.DeployERC20Remote(
			icttSrcDir,
			remoteRPCEndpoint,
			remoteKey,
			common.HexToAddress(remoteRegistryAddress),
			common.HexToAddress(remoteKeyAddress),
			homeBlockchainID,
			homeAddress,
			homeDecimals,
			tokenHomeName,
			tokenHomeSymbol,
			remoteDecimals,
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
			homeDecimals,
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
			return errors.New("timeout waiting for remote endpoint registration")
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

		if err := precompiles.SetEnabled(
			remoteRPCEndpoint,
			precompiles.NativeMinterPrecompile,
			remoteMinterManagerPrivKey,
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
				return errors.New("timeout waiting for remote endpoint collateralization")
			}
			time.Sleep(checkInterval)
		}

		if flags.remoteFlags.removeMinterAdmin && remoteMinterManagerIsAdmin {
			ux.Logger.PrintToUser("Removing minter admin %s", remoteMinterManagerAddress)
			if err := precompiles.SetNone(
				remoteRPCEndpoint,
				precompiles.NativeMinterPrecompile,
				remoteMinterManagerPrivKey,
				common.HexToAddress(remoteMinterManagerAddress),
			); err != nil {
				return err
			}
		} else {
			minterRole := "admin"
			if !remoteMinterManagerIsAdmin {
				minterRole = "manager"
			}
			ux.Logger.PrintToUser("Original minter %s %s is left in place", minterRole, remoteMinterManagerAddress)
		}
	}

	ux.Logger.PrintToUser("Remote Deployed to %s", remoteRPCEndpoint)
	ux.Logger.PrintToUser("Remote Address: %s", remoteAddress)

	return nil
}

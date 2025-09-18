// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/duallogger"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/libevm/common"

	"github.com/spf13/cobra"
)

type DeployValidatorManagerFlags struct {
	network              networkoptions.NetworkFlags
	privateKey           contract.PrivateKeyFlags
	proxyOwnerPrivateKey contract.PrivateKeyFlags
	chainFlags           contract.ChainSpec
	rpcEndpoint          string
	proxyAdmin           string
	proxy                string
	deployProxy          bool
	poa                  bool
	pos                  bool
	validatorManagerPath string
	rewardBasisPoints    uint64
}

var deployValidatorManagerFlags DeployValidatorManagerFlags

// avalanche contract deploy validatorManager
func newDeployValidatorManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validatorManager",
		Short: "Deploy a Validator Manager into a given Network and Blockchain",
		Long: `Deploy a Validator Manager, a Proxy, and a Proxy Admin, into a given Network and Blockchain.
If a proxy is provided, configures it to point to the deployed validator manager.
Note: This command deploys smart contracts for a validator manager, but does not initializate it to start operating on a given
L1. For that, you need to call 'avalanche contract initValidatorManager'.
`,
		RunE: deployValidatorManager,
		Args: cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployValidatorManagerFlags.network, true, networkoptions.DefaultSupportedNetworkOptions)
	deployValidatorManagerFlags.privateKey.AddToCmd(cmd, "as contract deployer")
	deployValidatorManagerFlags.proxyOwnerPrivateKey.SetFlagNames("proxy-owner-private-key", "proxy-owner-key", "proxy-owner-genesis-key")
	deployValidatorManagerFlags.proxyOwnerPrivateKey.AddToCmd(cmd, "as proxy owner")
	// enabling blockchain names, C-Chain and blockchain IDs
	deployValidatorManagerFlags.chainFlags.SetEnabled(true, true, false, false, true)
	deployValidatorManagerFlags.chainFlags.AddToCmd(cmd, "deploy a Validator Manager contract into %s")
	cmd.Flags().StringVar(&deployValidatorManagerFlags.rpcEndpoint, "rpc", "", "deploy the contract into the given rpc endpoint")
	cmd.Flags().StringVar(&deployValidatorManagerFlags.proxyAdmin, "proxy-admin", "", "use the given proxy admin")
	cmd.Flags().StringVar(&deployValidatorManagerFlags.proxy, "proxy", "", "use the given proxy")
	cmd.Flags().BoolVar(&deployValidatorManagerFlags.deployProxy, "deploy-proxy", false, "deploy a new proxy and admin for the validator manager")
	cmd.Flags().BoolVar(&deployValidatorManagerFlags.poa, "poa", false, "deploy a v2.0.0 Proof of Authority Validator Manager")
	cmd.Flags().BoolVar(&deployValidatorManagerFlags.pos, "pos", false, "deploy a v2.0.0 Proof of Stake Validator Manager")
	cmd.Flags().StringVar(&deployValidatorManagerFlags.validatorManagerPath, "validator-manager-path", "", "deploy the validator manager contained in the given path (hex encoded)")
	cmd.Flags().Uint64Var(&deployValidatorManagerFlags.rewardBasisPoints, "reward-basis-points", 100, "(PoS only) reward basis points for PoS Reward Calculator")
	return cmd
}

func deployValidatorManager(cmd *cobra.Command, _ []string) error {
	return CallDeployValidatorManager(cmd, deployValidatorManagerFlags)
}

func CallDeployValidatorManager(cmd *cobra.Command, flags DeployValidatorManagerFlags) error {
	if !flags.poa && !flags.pos && flags.validatorManagerPath == "" {
		customOption := "Custom"
		options := []string{validatormanagertypes.ProofOfAuthority, validatormanagertypes.ProofOfStake, customOption}
		option, err := app.Prompt.CaptureList(
			"Which validator manager do you want to deploy?",
			options,
		)
		if err != nil {
			return err
		}
		switch option {
		case validatormanagertypes.ProofOfAuthority:
			flags.poa = true
		case validatormanagertypes.ProofOfStake:
			flags.pos = true
		case customOption:
			flags.validatorManagerPath, err = app.Prompt.CaptureExistingFilepath("Provide filepath that contains validator manager bytecode encoded as hexa:")
			if err != nil {
				return err
			}
		}
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		flags.network,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	if err := flags.chainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}
	if !flags.chainFlags.Defined() {
		prompt := "Where do you want to Deploy the Validator Manager?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			"",
			&flags.chainFlags,
		); cancel || err != nil {
			return err
		}
	}
	if flags.rpcEndpoint == "" {
		flags.rpcEndpoint, _, err = contract.GetBlockchainEndpoints(
			app,
			network,
			flags.chainFlags,
			true,
			false,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), flags.rpcEndpoint)
	}
	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		flags.chainFlags,
	)
	if err != nil {
		return err
	}
	privateKey, err := flags.privateKey.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		ux.Logger.PrintToUser("A private key is needed to pay for the contract deploy fees.")
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"deploy the contract",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}

	if flag := cmd.Flags().Lookup("deploy-proxy"); flag != nil && !flag.Changed {
		flags.deployProxy, err = app.Prompt.CaptureNoYes("Do you want to deploy a new proxy altogether with the validator manager?")
		if err != nil {
			return err
		}
	}

	if flags.deployProxy && (flags.proxyAdmin != "" || flags.proxy != "") {
		return fmt.Errorf("can't ask to deploy a proxy while providing either proxy admin or proxy address as input")
	}

	if !flags.deployProxy {
		if flags.proxy == "" {
			addr, err := app.Prompt.CaptureAddress("Which is the proxy contract address?")
			if err != nil {
				return err
			}
			flags.proxy = addr.Hex()
		}
		if flags.proxyAdmin == "" {
			addr, err := app.Prompt.CaptureAddress("Which is the proxy admin contract address?")
			if err != nil {
				return err
			}
			flags.proxyAdmin = addr.Hex()
		}
	}

	proxyOwnerPrivateKey, err := flags.proxyOwnerPrivateKey.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if proxyOwnerPrivateKey == "" {
		if flags.deployProxy {
			ux.Logger.PrintToUser("A private key is needed as owner of the new proxy")
		}
		proxyOwnerPrivateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"set up the proxy",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("Deploying validator manager contract")
	var validatorManagerAddress common.Address
	switch {
	case flags.poa:
		validatorManagerAddress, _, _, err = validatormanager.DeployValidatorManagerV2_0_0Contract(
			flags.rpcEndpoint,
			privateKey,
			false,
		)
		if err != nil {
			return err
		}
	case flags.pos:
		rewardCalculatorAddress, _, _, err := validatormanager.DeployRewardCalculatorV2_0_0Contract(
			flags.rpcEndpoint,
			privateKey,
			flags.rewardBasisPoints,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Reward Calculator Address: %s", rewardCalculatorAddress.Hex())
		validatorManagerAddress, _, _, err = validatormanager.DeployPoSValidatorManagerV2_0_0Contract(
			flags.rpcEndpoint,
			privateKey,
			false,
		)
		if err != nil {
			return err
		}
	case flags.validatorManagerPath != "":
		bytecode, err := os.ReadFile(flags.validatorManagerPath)
		if err != nil {
			return err
		}
		validatorManagerAddress, _, _, err = validatormanager.DeployValidatorManagerContract(
			flags.rpcEndpoint,
			privateKey,
			false,
			string(bytecode),
		)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Validator Manager Address: %s", validatorManagerAddress.Hex())
	ux.Logger.PrintToUser("")

	if !flags.deployProxy {
		ux.Logger.PrintToUser("Updating proxy")
		_, _, err = validatormanager.SetupProxyImplementation(
			duallogger.NewDualLogger(true, app),
			flags.rpcEndpoint,
			common.HexToAddress(flags.proxyAdmin),
			common.HexToAddress(flags.proxy),
			proxyOwnerPrivateKey,
			validatorManagerAddress,
			"setup deployed VMC at proxy",
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Proxy successfully configured")
		return nil
	}

	ux.Logger.PrintToUser("Deploying proxy contracts")

	proxyOwnerAddress, err := evm.PrivateKeyToAddress(proxyOwnerPrivateKey)
	if err != nil {
		return err
	}
	proxy, proxyAdmin, _, _, err := validatormanager.DeployTransparentProxy(
		flags.rpcEndpoint,
		privateKey,
		validatorManagerAddress,
		proxyOwnerAddress,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Proxy Address: %s", proxy.Hex())
	ux.Logger.PrintToUser("Proxy Admin Address: %s", proxyAdmin.Hex())
	return nil
}

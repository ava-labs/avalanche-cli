// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

type DeployValidatorManagerFlags struct {
	network              networkoptions.NetworkFlags
	privateKey           contract.PrivateKeyFlags
	proxyOwnerPrivateKey contract.PrivateKeyFlags
	chainFlags           contract.ChainSpec
	rpcEndpoint          string
	proxyAdmin           string
	transparentProxy     string
}

var deployValidatorManagerFlags DeployValidatorManagerFlags

// avalanche contract deploy validatorManager
func newDeployValidatorManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validatorManager",
		Short: "Deploy a Validator Manager into a given Network and Blockchain",
		Long:  "Deploy a Validator Manager into a given Network and Blockchain",
		RunE:  deployValidatorManager,
		Args:  cobrautils.ExactArgs(0),
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
	cmd.Flags().StringVar(&deployValidatorManagerFlags.transparentProxy, "transparent-proxy", "", "use the given transparent proxy")
	return cmd
}

func deployValidatorManager(_ *cobra.Command, _ []string) error {
	return CallDeployValidatorManager(deployValidatorManagerFlags)
}

func CallDeployValidatorManager(flags DeployValidatorManagerFlags) error {
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
	proxyOwnerPrivateKey, err := flags.proxyOwnerPrivateKey.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if proxyOwnerPrivateKey == "" {
		ux.Logger.PrintToUser("A private key is needed as owner of the validator manager proxy")
		proxyOwnerPrivateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"own the validator manager proxy",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	// TODO: ask for confirmation for the full set of operations
	var proxyAdminAddress common.Address
	if flags.proxyAdmin == "" {
		proxyOwnerAddress, err := evm.PrivateKeyToAddress(proxyOwnerPrivateKey)
		if err != nil {
			return err
		}
		proxyAdminAddress, _, _, err = validatormanager.DeployProxyAdmin(
			flags.rpcEndpoint,
			privateKey,
			proxyOwnerAddress,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Proxy Admin Address: %s", proxyAdminAddress.Hex())
	} else {
		proxyAdminAddress = common.HexToAddress(flags.proxyAdmin)
	}
	validatorManagerAddress, _, _, err := validatormanager.DeployValidatorManagerV2_0_0Contract(
			flags.rpcEndpoint,
			privateKey,
			false,
	)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Validator Manager Address: %s", validatorManagerAddress.Hex())
	var transparentProxyAddress common.Address 
	if flags.transparentProxy == "" {
		proxyOwnerAddress, err := evm.PrivateKeyToAddress(proxyOwnerPrivateKey)
		if err != nil {
			return err
		}
		var receipt *types.Receipt
		transparentProxyAddress, _, receipt, err = validatormanager.DeployTransparentProxy(
			flags.rpcEndpoint,
			privateKey,
			validatorManagerAddress,
			proxyOwnerAddress,
		)
		if err != nil {
			return err
		}
		event, err := evm.GetEventFromLogs(receipt.Logs, validatormanager.ParseAdminChanged)
		if err != nil {
			return err
		}
		proxyAdminAddress = event.NewAdmin
		fmt.Printf("%#v\n", event)
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Transparent Proxy Address: %s", transparentProxyAddress.Hex())
	} else {
		transparentProxyAddress = common.HexToAddress(flags.transparentProxy)
	}
	_, _, err = validatormanager.SetupProxyImplementation(
		flags.rpcEndpoint,
		proxyAdminAddress,
		transparentProxyAddress,
		proxyOwnerPrivateKey,
		validatorManagerAddress,
		"test",
	)
	if err != nil {
		return err
	}
	return nil
}

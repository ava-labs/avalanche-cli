// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"math"
	"os"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/dependencies"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/libevm/common"

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
		return constants.ErrNoSubnetID
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

func StartLocalMachine(
	network models.Network,
	sidecar models.Sidecar,
	blockchainName string,
	deployBalance,
	availableBalance uint64,
	localMachineFlags *flags.LocalMachineFlags,
	bootstrapValidatorFlags *flags.BootstrapValidatorFlags,
) (bool, error) {
	var err error
	if network.Kind == models.Local &&
		!bootstrapValidatorFlags.GenerateNodeID &&
		bootstrapValidatorFlags.BootstrapEndpoints == nil &&
		bootstrapValidatorFlags.BootstrapValidatorsJSONFilePath == "" {
		localMachineFlags.UseLocalMachine = true
	}
	clusterName := localnet.LocalClusterName(network, blockchainName)
	if clusterNameFlagValue != "" {
		clusterName = clusterNameFlagValue
		if localnet.LocalClusterExists(app, clusterName) {
			localMachineFlags.UseLocalMachine = true
			if len(bootstrapValidatorFlags.BootstrapEndpoints) == 0 {
				bootstrapValidatorFlags.BootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
				if err != nil {
					return false, fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
						"please create your local node again and call blockchain deploy command again", err)
				}
			}
			network = models.ConvertClusterToNetwork(network)
		}
	}
	// ask user if we want to use local machine if cluster is not provided
	if !localMachineFlags.UseLocalMachine && clusterNameFlagValue == "" {
		ux.Logger.PrintToUser("You can use your local machine as a bootstrap validator on the blockchain")
		ux.Logger.PrintToUser("This means that you don't have to to set up a remote server on a cloud service (e.g. AWS / GCP) to be a validator on the blockchain.")

		localMachineFlags.UseLocalMachine, err = app.Prompt.CaptureYesNo("Do you want to use your local machine as a bootstrap validator?")
		if err != nil {
			return false, err
		}
	}
	// default number of local machine nodes to be 1
	if localMachineFlags.UseLocalMachine && bootstrapValidatorFlags.NumBootstrapValidators == 0 {
		bootstrapValidatorFlags.NumBootstrapValidators = constants.DefaultNumberOfLocalMachineNodes
	}
	connectionSettings := localnet.ConnectionSettings{}
	if network.Kind == models.Granite {
		connectionSettings = node.GetGraniteConnectionSettings()
	}
	// if no cluster provided - we create one with fmt.Sprintf("%s-local-node-%s", blockchainName, networkNameComponent) name
	if localMachineFlags.UseLocalMachine && clusterNameFlagValue == "" {
		if localnet.LocalClusterExists(app, clusterName) {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser(
				logging.Red.Wrap("A local machine L1 deploy already exists for %s L1 and network %s"),
				blockchainName,
				network.Name(),
			)
			yes, err := app.Prompt.CaptureNoYes(
				fmt.Sprintf("Do you want to overwrite the current local L1 deploy for %s?", blockchainName),
			)
			if err != nil {
				return false, err
			}
			if !yes {
				return true, nil
			}
			_ = localnet.LocalClusterRemove(app, clusterName)
			ux.Logger.GreenCheckmarkToUser("Local node %s cleaned up.", clusterName)
		}
		requiredBalance := deployBalance * uint64(bootstrapValidatorFlags.NumBootstrapValidators)
		if availableBalance < requiredBalance {
			return false, fmt.Errorf(
				"required balance for %d validators dynamic fee on PChain is %d but the given key has %d",
				bootstrapValidatorFlags.NumBootstrapValidators,
				requiredBalance,
				availableBalance,
			)
		}
		avagoVersionSettings := dependencies.AvalancheGoVersionSettings{}
		// setup (install if needed) avalanchego binary
		avagoVersion := localMachineFlags.UserProvidedAvagoVersion
		if localMachineFlags.UserProvidedAvagoVersion == constants.DefaultAvalancheGoVersion && localMachineFlags.AvagoBinaryPath == "" {
			// nothing given: get avago version from RPC compat using latest.json defined in
			// https://raw.githubusercontent.com/ava-labs/avalanche-cli/control-default-version/versions/latest.json
			avagoVersion, err = dependencies.GetLatestCLISupportedDependencyVersion(app, constants.AvalancheGoRepoName, network, &sidecar.RPCVersion)
			if err != nil {
				if err != dependencies.ErrNoAvagoVersion {
					return false, err
				}
				avagoVersion = constants.LatestPreReleaseVersionTag
			}
		}
		localMachineFlags.AvagoBinaryPath, err = localnet.SetupAvalancheGoBinary(app, avagoVersion, localMachineFlags.AvagoBinaryPath)
		if err != nil {
			return false, err
		}
		nodeConfig := map[string]interface{}{}
		if partialSync {
			nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
		}
		if network.Kind == models.Fuji {
			globalNetworkFlags.UseFuji = true
		}
		if network.Kind == models.Mainnet {
			globalNetworkFlags.UseMainnet = true
		}
		nodeSettingsLen := max(len(localMachineFlags.StakingSignerKeyPaths), len(localMachineFlags.HTTPPorts), len(localMachineFlags.StakingPorts))
		nodeSettings := make([]localnet.NodeSetting, nodeSettingsLen)
		for i := range nodeSettingsLen {
			nodeSetting := localnet.NodeSetting{}
			if i < len(localMachineFlags.StakingSignerKeyPaths) {
				stakingSignerKey, err := os.ReadFile(localMachineFlags.StakingSignerKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking signer key at %s: %w", localMachineFlags.StakingSignerKeyPaths[i], err)
				}
				stakingCertKey, err := os.ReadFile(localMachineFlags.StakingCertKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking cert key at %s: %w", localMachineFlags.StakingCertKeyPaths[i], err)
				}
				stakingTLSKey, err := os.ReadFile(localMachineFlags.StakingTLSKeyPaths[i])
				if err != nil {
					return false, fmt.Errorf("could not read staking TLS key at %s: %w", localMachineFlags.StakingTLSKeyPaths[i], err)
				}
				nodeSetting.StakingSignerKey = stakingSignerKey
				nodeSetting.StakingCertKey = stakingCertKey
				nodeSetting.StakingTLSKey = stakingTLSKey
			}
			if i < len(localMachineFlags.HTTPPorts) {
				nodeSetting.HTTPPort = uint64(localMachineFlags.HTTPPorts[i])
			}
			if i < len(localMachineFlags.StakingPorts) {
				nodeSetting.StakingPort = uint64(localMachineFlags.StakingPorts[i])
			}
			nodeSettings[i] = nodeSetting
		}
		if bootstrapValidatorFlags.NumBootstrapValidators > math.MaxUint32 {
			return false, fmt.Errorf("too many bootstrap validators")
		}
		// anrSettings, avagoVersionSettings, globalNetworkFlags are empty
		if err = node.StartLocalNode(
			app,
			clusterName,
			localMachineFlags.AvagoBinaryPath,
			vm.EvmDebugConfig,
			uint32(bootstrapValidatorFlags.NumBootstrapValidators),
			nodeConfig,
			connectionSettings,
			nodeSettings,
			avagoVersionSettings,
			network,
		); err != nil {
			return false, err
		}
		clusterNameFlagValue = clusterName
		if len(bootstrapValidatorFlags.BootstrapEndpoints) == 0 {
			bootstrapValidatorFlags.BootstrapEndpoints, err = localnet.GetLocalClusterURIs(app, clusterName)
			if err != nil {
				return false, fmt.Errorf("error getting local host bootstrap endpoints: %w, "+
					"please create your local node again and call blockchain deploy command again", err)
			}
		}
	}
	return false, nil
}

func CompleteValidatorManagerL1Deploy(
	logger logging.Logger,
	network models.Network,
	blockchainName string,
	validatorManagerRPCEndpoint string,
	proxyContractOwner string,
	isPoS bool,
	useACP99 bool,
) error {
	if isPoS {
		deployed, err := validatormanager.GenesisValidatorProxyHasImplementationSet(validatorManagerRPCEndpoint)
		if err != nil {
			return err
		}
		if !deployed {
			_, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
				app,
				network,
				contract.ChainSpec{
					BlockchainName: blockchainName,
				},
			)
			if err != nil {
				return err
			}
			// it is not in genesis
			ux.Logger.PrintToUser("Deploying Proof of Stake Validator Manager contract on blockchain %s ...", blockchainName)
			proxyOwnerPrivateKey, err := GetProxyOwnerPrivateKey(
				app,
				network,
				proxyContractOwner,
				ux.Logger.PrintToUser,
			)
			if err != nil {
				return err
			}
			genesisSigner, err := evm.NewSignerFromPrivateKey(genesisPrivateKey)
			if err != nil {
				return err
			}
			proxyOwnerSigner, err := evm.NewSignerFromPrivateKey(proxyOwnerPrivateKey)
			if err != nil {
				return err
			}
			if useACP99 {
				_, err := validatormanager.DeployValidatorManagerV2_0_0ContractAndRegisterAtGenesisProxy(
					logger,
					validatorManagerRPCEndpoint,
					genesisSigner,
					true,
					proxyOwnerSigner,
				)
				if err != nil {
					return err
				}
				_, err = validatormanager.DeployPoSValidatorManagerV2_0_0ContractAndRegisterAtGenesisProxy(
					logger,
					validatorManagerRPCEndpoint,
					genesisSigner,
					true,
					proxyOwnerSigner,
				)
				if err != nil {
					return err
				}
			} else {
				if _, err := validatormanager.DeployPoSValidatorManagerV1_0_0ContractAndRegisterAtGenesisProxy(
					logger,
					validatorManagerRPCEndpoint,
					genesisSigner,
					true,
					proxyOwnerSigner,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

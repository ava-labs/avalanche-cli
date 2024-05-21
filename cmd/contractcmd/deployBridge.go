// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

type DeployFlags struct {
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

const (
	cChainAlias = "C"
	cChainName  = "c-chain"
)

var (
	deployBridgeSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployFlags   DeployFlags
	foundryupPath = utils.ExpandHome("~/.foundry/bin/foundryup")
	forgePath     = utils.ExpandHome("~/.foundry/bin/forge")
)

func foundryIsInstalled() bool {
	return utils.IsExecutable(forgePath)
}

func installFoundry() error {
	ux.Logger.PrintToUser("Installing Foundry")
	downloadCmd := exec.Command("curl", "-L", "https://foundry.paradigm.xyz")
	installCmd := exec.Command("sh")
	var downloadOutbuf, downloadErrbuf strings.Builder
	downloadCmdStdoutPipe, err := downloadCmd.StdoutPipe()
	if err != nil {
		return err
	}
	downloadCmd.Stderr = &downloadErrbuf
	installCmd.Stdin = io.TeeReader(downloadCmdStdoutPipe, &downloadOutbuf)
	var installOutbuf, installErrbuf strings.Builder
	installCmd.Stdout = &installOutbuf
	installCmd.Stderr = &installErrbuf
	if err := installCmd.Start(); err != nil {
		return err
	}
	if err := downloadCmd.Run(); err != nil {
		if downloadOutbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadOutbuf.String(), "\n"))
		}
		if downloadErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(downloadErrbuf.String(), "\n"))
		}
		return err
	}
	if err := installCmd.Wait(); err != nil {
		if installOutbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n"))
		}
		if installErrbuf.String() != "" {
			ux.Logger.PrintToUser(strings.TrimSuffix(installErrbuf.String(), "\n"))
		}
		ux.Logger.PrintToUser("installation failed: %s", err.Error())
		return err
	}
	ux.Logger.PrintToUser(strings.TrimSuffix(installOutbuf.String(), "\n"))
	out, err := exec.Command(foundryupPath).CombinedOutput()
	ux.Logger.PrintToUser(string(out))
	if err != nil {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Foundry toolset is not available and couldn't automatically be installed. It is a necessary dependency for CLI to compile smart contracts.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://book.getfoundry.sh/getting-started/installation and try again")
		ux.Logger.PrintToUser("")
	}
	return err
}

// avalanche contract deploy bridge
func newDeployBridgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Deploys Tokeb Bridge into a given Network and Subnets",
		Long:  "Deploys Tokeb Bridge into a given Network and Subnets",
		RunE:  deployBridge,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deployBridgeSupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.SubnetName, "subnet", "", "deploy teleporter into the given CLI subnet")
	cmd.Flags().StringVar(&deployFlags.BlockchainID, "blockchain-id", "", "deploy teleporter into the given blockchain ID/Alias")
	cmd.Flags().BoolVar(&deployFlags.CChain, "c-chain", false, "deploy teleporter into C-Chain")
	cmd.Flags().StringVar(&deployFlags.PrivateKey, "private-key", "", "private key to use to fund teleporter deploy)")
	cmd.Flags().StringVar(&deployFlags.KeyName, "key", "", "CLI stored key to use to fund teleporter deploy)")
	cmd.Flags().BoolVar(&deployFlags.GenesisKey, "genesis-key", false, "use genesis aidrop key to fund teleporter deploy")
	cmd.Flags().BoolVar(&deployFlags.DeployMessenger, "deploy-messenger", true, "deploy Teleporter Messenger")
	cmd.Flags().BoolVar(&deployFlags.DeployRegistry, "deploy-registry", true, "deploy Teleporter Registry")
	cmd.Flags().StringVar(&deployFlags.TeleporterVersion, "version", "latest", "version to deploy")
	cmd.Flags().StringVar(&deployFlags.RPCURL, "rpc-url", "", "use the given RPC URL to connect to the subnet")
	return cmd
}

func deployBridge(_ *cobra.Command, args []string) error {
	return CallDeployBridge(args, deployFlags)
}

func test(
	rpcURL string,
	prefundedPrivateKey string,
) error {
	srcDir := utils.ExpandHome("~/Workspace/projects/teleporter-token-bridge/")
	abiPath := filepath.Join(srcDir, "contracts/out/WrappedNativeToken.sol/WrappedNativeToken.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/WrappedNativeToken.sol/WrappedNativeToken.bin")
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return err
	}
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return err
	}
	metadata := &bind.MetaData{
		ABI: string(abiBytes),
		Bin: string(binBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return err
	}
	bin := common.FromHex(metadata.Bin)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return err
	}
	address, tx, _, err := bind.DeployContract(txOpts, *abi, bin, client, "Al fin", "ALFIN")
	if err != nil {
		return err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return err
	} else if !success {
		return fmt.Errorf("failed receipt status deploying contract")
	}
	fmt.Println(address)
	return nil
}

func CallDeployBridge(_ []string, flags DeployFlags) error {
	if err := vm.CheckGitIsInstalled(); err != nil {
		return err
	}
	if !foundryIsInstalled() {
		return installFoundry()
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter bridge?",
		flags.Network,
		true,
		false,
		deployBridgeSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	return test(
		network.BlockchainEndpoint("2FZA3PDpQvYy6uevt34xr7Sv4RczKe3827PWPqAymfqXhJkkGL"),
		"6e6cb03f2f64e298b28e56bc53a051257bff62be978b6df010fce46a8fdde2cb",
	)
	bridgeSrcDir := utils.ExpandHome("~/Workspace/projects/teleporter-token-bridge/")
	// install bridge src dependencies
	cmd := exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	cmd.Dir = bridgeSrcDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		ux.Logger.PrintToUser(string(out))
		return err
	}
	// build teleporter contracts bytecode + abi
	cmd = exec.Command(
		forgePath,
		"build",
		"--extra-output-files",
		"abi",
		"bin",
	)
	cmd.Dir = filepath.Join(bridgeSrcDir, "contracts")
	out, err = cmd.CombinedOutput()
	if err != nil {
		ux.Logger.PrintToUser(string(out))
		return err
	}
	return nil
	cmd = exec.Command(
		forgePath,
		"install",
		"openzeppelin/openzeppelin-contracts@v4.8.1",
		"--no-commit",
	)
	cmd.Dir = utils.ExpandHome("~/Workspace/projects/avalanche-cli/")
	out, err = cmd.CombinedOutput()
	fmt.Println(string(out))
	fmt.Println(err)
	createCmd := exec.Command(
		forgePath,
		"create",
		"--rpc-url",
		network.BlockchainEndpoint("2FZA3PDpQvYy6uevt34xr7Sv4RczKe3827PWPqAymfqXhJkkGL"),
		"--private-key",
		"6e6cb03f2f64e298b28e56bc53a051257bff62be978b6df010fce46a8fdde2cb",
		"src/5-native-token-bridge/WrappedNativeToken.sol:WrappedNativeToken",
		"--constructor-args",
		"Wrapped TOK Name",
		"WTOK",
	)
	createCmd.Dir = utils.ExpandHome("~/Workspace/projects/avalanche-cli/")
	out, err = createCmd.CombinedOutput()
	fmt.Println(string(out))
	fmt.Println(err)
	return nil
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.SubnetName != "", flags.BlockchainID != "", flags.CChain}) {
		return fmt.Errorf("--subnet, --blockchain-id and --cchain are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.PrivateKey != "", flags.KeyName != "", flags.GenesisKey}) {
		return fmt.Errorf("--private-key, --key and --genesis-key are mutually exclusive flags")
	}
	if !flags.DeployMessenger && !flags.DeployRegistry {
		return fmt.Errorf("you should set at least one of --deploy-messenger/--deploy-registry to true")
	}
	if flags.SubnetName == "" && flags.BlockchainID == "" && !flags.CChain {
		// fill flags based on user prompts
		blockchainIDOptions := []string{
			"Get Blockchain ID from an existing subnet (deployed with avalanche subnet deploy)",
			"Use C-Chain Blockchain ID",
			"Custom",
		}
		blockchainIDOption, err := app.Prompt.CaptureList("Which Blockchain ID would you like to deploy Teleporter to?", blockchainIDOptions)
		if err != nil {
			return err
		}
		switch blockchainIDOption {
		case blockchainIDOptions[0]:
			subnetNames, err := app.GetSubnetNames()
			if err != nil {
				return err
			}
			flags.SubnetName, err = app.Prompt.CaptureList(
				"Choose a Subnet",
				subnetNames,
			)
			if err != nil {
				return err
			}
		case blockchainIDOptions[1]:
			flags.CChain = true
		default:
			flags.BlockchainID, err = app.Prompt.CaptureString("Blockchain ID/Alias")
			if err != nil {
				return err
			}
		}
	}

	var (
		blockchainID         string
		teleporterSubnetDesc string
		privateKey           = flags.PrivateKey
		teleporterVersion    string
	)
	switch {
	case flags.SubnetName != "":
		teleporterSubnetDesc = flags.SubnetName
		sc, err := app.LoadSidecar(flags.SubnetName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		if b, _, err := subnetcmd.HasSubnetEVMGenesis(flags.SubnetName); err != nil {
			return err
		} else if !b {
			return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter")
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return fmt.Errorf("subnet has not been deployed to %s", network.Name())
		}
		blockchainID = sc.Networks[network.Name()].BlockchainID.String()
		if sc.TeleporterVersion != "" {
			teleporterVersion = sc.TeleporterVersion
		}
		if sc.TeleporterKey != "" {
			k, err := app.GetKey(sc.TeleporterKey, network, true)
			if err != nil {
				return err
			}
			privateKey = k.PrivKeyHex()
		}
	case flags.BlockchainID != "":
		teleporterSubnetDesc = flags.BlockchainID
		blockchainID = flags.BlockchainID
	case flags.CChain:
		teleporterSubnetDesc = cChainName
		blockchainID = cChainAlias
	}
	var chainID ids.ID
	if flags.CChain || !network.StandardPublicEndpoint() {
		chainID, err = utils.GetChainID(network.Endpoint, blockchainID)
		if err != nil {
			return err
		}
	} else {
		chainID, err = ids.FromString(blockchainID)
		if err != nil {
			return err
		}
	}
	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, chainID)
	if err != nil {
		return err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(createChainTx.GenesisData) {
		return fmt.Errorf("teleporter can only be deployed to Subnet-EVM based vms")
	}
	if flags.KeyName != "" {
		k, err := app.GetKey(flags.KeyName, network, false)
		if err != nil {
			return err
		}
		privateKey = k.PrivKeyHex()
	}
	_, genesisAddress, genesisPrivateKey, err := subnet.GetSubnetAirdropKeyInfo(app, network, flags.SubnetName, createChainTx.GenesisData)
	if err != nil {
		return err
	}
	if flags.GenesisKey {
		privateKey = genesisPrivateKey
	}
	if privateKey == "" {
		cliKeyOpt := "Get private key from an existing stored key (created from avalanche key create or avalanche key import)"
		customKeyOpt := "Custom"
		genesisKeyOpt := fmt.Sprintf("Use the private key of the Genesis Aidrop address %s", genesisAddress)
		keyOptions := []string{cliKeyOpt, customKeyOpt}
		if genesisPrivateKey != "" {
			keyOptions = []string{genesisKeyOpt, cliKeyOpt, customKeyOpt}
		}
		keyOption, err := app.Prompt.CaptureList("Which private key do you want to use to pay fees?", keyOptions)
		if err != nil {
			return err
		}
		switch keyOption {
		case cliKeyOpt:
			keyName, err := prompts.CaptureKeyName(app.Prompt, "pay fees", app.GetKeyDir(), true)
			if err != nil {
				return err
			}
			k, err := app.GetKey(keyName, network, false)
			if err != nil {
				return err
			}
			privateKey = k.PrivKeyHex()
		case customKeyOpt:
			privateKey, err = app.Prompt.CaptureString("Private Key")
			if err != nil {
				return err
			}
		case genesisKeyOpt:
			privateKey = genesisPrivateKey
		}
	}
	if flags.TeleporterVersion != "" && flags.TeleporterVersion != "latest" {
		teleporterVersion = flags.TeleporterVersion
	} else if teleporterVersion == "" {
		teleporterInfo, err := teleporter.GetInfo(app)
		if err != nil {
			return err
		}
		teleporterVersion = teleporterInfo.Version
	}
	// deploy to subnet
	rpcURL := network.BlockchainEndpoint(blockchainID)
	if flags.RPCURL != "" {
		rpcURL = flags.RPCURL
	}
	td := teleporter.Deployer{}
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
		app.GetTeleporterBinDir(),
		teleporterVersion,
		teleporterSubnetDesc,
		rpcURL,
		privateKey,
		flags.DeployMessenger,
		flags.DeployRegistry,
	)
	if err != nil {
		return err
	}
	if flags.SubnetName != "" && !alreadyDeployed {
		// update sidecar
		sc, err := app.LoadSidecar(flags.SubnetName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		sc.TeleporterReady = true
		sc.TeleporterVersion = teleporterVersion
		networkInfo := sc.Networks[network.Name()]
		if teleporterMessengerAddress != "" {
			networkInfo.TeleporterMessengerAddress = teleporterMessengerAddress
		}
		if teleporterRegistryAddress != "" {
			networkInfo.TeleporterRegistryAddress = teleporterRegistryAddress
		}
		sc.Networks[network.Name()] = networkInfo
		if err := app.UpdateSidecar(&sc); err != nil {
			return err
		}
	}
	// automatic deploy to cchain for local/devnet
	if !flags.CChain && (network.Kind == models.Local || network.Kind == models.Devnet) {
		ewoq, err := app.GetKey("ewoq", network, false)
		if err != nil {
			return err
		}
		alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
			app.GetTeleporterBinDir(),
			teleporterVersion,
			cChainName,
			network.BlockchainEndpoint(cChainAlias),
			ewoq.PrivKeyHex(),
			flags.DeployMessenger,
			flags.DeployRegistry,
		)
		if err != nil {
			return err
		}
		if !alreadyDeployed {
			if network.Kind == models.Local {
				if err := subnet.WriteExtraLocalNetworkData(app, teleporterMessengerAddress, teleporterRegistryAddress); err != nil {
					return err
				}
			}
			if network.ClusterName != "" {
				clusterConfig, err := app.GetClusterConfig(network.ClusterName)
				if err != nil {
					return err
				}
				if teleporterMessengerAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress = teleporterMessengerAddress
				}
				if teleporterRegistryAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress = teleporterRegistryAddress
				}
				if err := app.SetClusterConfig(network.ClusterName, clusterConfig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

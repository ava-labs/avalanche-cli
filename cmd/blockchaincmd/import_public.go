// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/rpc"
	"github.com/ava-labs/coreth/core"
	"github.com/spf13/cobra"
)

var (
	importPublicSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Fuji, networkoptions.Mainnet}
	blockchainIDstr                     string
	nodeURL                             string
	useSubnetEvm                        bool
	useCustomVM                         bool
)

// avalanche subnet import public
func newImportPublicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "public [subnetPath]",
		Short: "Import an existing subnet config from running subnets on a public network",
		RunE:  importPublic,
		Args:  cobrautils.MaximumNArgs(1),
		Long: `The subnet import public command imports a Subnet configuration from a running network.

The genesis file should be available from the disk for this to work. By default, an imported Subnet
doesn't overwrite an existing Subnet with the same name. To allow overwrites, provide the --force
flag.`,
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, importPublicSupportedNetworkOptions)

	cmd.Flags().StringVar(&nodeURL, "node-url", "", "[optional] URL of an already running subnet validator")

	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "import a subnet-evm")
	cmd.Flags().BoolVar(&useCustomVM, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(
		&overwriteImport,
		"force",
		false,
		"overwrite the existing configuration if one exists",
	)
	cmd.Flags().StringVar(
		&blockchainIDstr,
		"blockchain-id",
		"",
		"the blockchain ID",
	)
	return cmd
}

func importPublic(*cobra.Command, []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		importPublicSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	var reply *info.GetNodeVersionReply

	if nodeURL == "" {
		yes, err := app.Prompt.CaptureNoYes("Have validator nodes already been deployed to this subnet?")
		if err != nil {
			return err
		}
		if yes {
			nodeURL, err = app.Prompt.CaptureString(
				"Please provide an API URL of such a node so we can query its VM version (e.g. http://111.22.33.44:5555)")
			if err != nil {
				return err
			}
			ctx, cancel := utils.GetAPIContext()
			defer cancel()
			infoAPI := info.NewClient(nodeURL)
			options := []rpc.Option{}
			reply, err = infoAPI.GetNodeVersion(ctx, options...)
			if err != nil {
				return fmt.Errorf("failed to query node - is it running and reachable? %w", err)
			}
		}
	}

	var blockchainID ids.ID
	if blockchainIDstr == "" {
		blockchainID, err = app.Prompt.CaptureID("What is the ID of the blockchain?")
		if err != nil {
			return err
		}
	} else {
		blockchainID, err = ids.FromString(blockchainIDstr)
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("Getting information from the %s network...", network.Name())

	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
	if err != nil {
		return err
	}

	vmID := createChainTx.VMID
	subnetID := createChainTx.SubnetID
	subnetName := createChainTx.ChainName
	genBytes := createChainTx.GenesisData

	ux.Logger.PrintToUser("Retrieved information. BlockchainID: %s, SubnetID: %s, Name: %s, VMID: %s",
		blockchainID.String(),
		subnetID.String(),
		subnetName,
		vmID.String(),
	)
	// TODO: it's probably possible to deploy VMs with the same name on a public network
	// In this case, an import could clash because the tool supports unique names only

	vmType, err := vm.PromptVMType(app, useSubnetEvm, useCustomVM)
	if err != nil {
		return err
	}

	vmIDstr := vmID.String()

	sc := &models.Sidecar{
		Name: subnetName,
		VM:   vmType,
		Networks: map[string]models.NetworkData{
			network.Name(): {
				SubnetID:     subnetID,
				BlockchainID: blockchainID,
			},
		},
		Subnet:       subnetName,
		Version:      constants.SidecarVersion,
		TokenName:    constants.DefaultTokenName,
		TokenSymbol:  constants.DefaultTokenSymbol,
		ImportedVMID: vmIDstr,
	}

	var versions []string

	if reply != nil {
		// a node was queried
		for _, v := range reply.VMVersions {
			if v == vmIDstr {
				sc.VMVersion = v
				break
			}
		}
		sc.RPCVersion = int(reply.RPCProtocolVersion)
	} else {
		// no node was queried, ask the user
		switch vmType {
		case models.SubnetEvm:
			versions, err = app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName)
			if err != nil {
				return err
			}
			sc.VMVersion, err = app.Prompt.CaptureList("Pick the version for this VM", versions)
		case models.CustomVM:
			return fmt.Errorf("importing custom VMs is not yet implemented, but will be available soon")
		default:
			return fmt.Errorf("unexpected VM type: %v", vmType)
		}
		if err != nil {
			return err
		}
		sc.RPCVersion, err = vm.GetRPCProtocolVersion(app, vmType, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed getting RPCVersion for VM type %s with version %s", vmType, sc.VMVersion)
		}
	}
	if vmType == models.SubnetEvm {
		var genesis core.Genesis
		if err := json.Unmarshal(genBytes, &genesis); err != nil {
			return err
		}
		sc.ChainID = genesis.Config.ChainID.String()
	}

	if err := app.CreateSidecar(sc); err != nil {
		return fmt.Errorf("failed creating the sidecar for import: %w", err)
	}

	if err = app.WriteGenesisFile(subnetName, genBytes); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Subnet %q imported successfully", sc.Name)

	return nil
}

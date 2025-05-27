// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/precompiles"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/rpc"
	"github.com/ava-labs/coreth/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	blockchainIDStr string
	nodeURL         string
	useSubnetEvm    bool
	useCustomVM     bool
)

// avalanche blockchain import public
func newImportPublicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "public [blockchainPath]",
		Short: "Import an existing blockchain config from running blockchains on a public network",
		RunE:  importPublic,
		Args:  cobrautils.MaximumNArgs(1),
		Long: `The blockchain import public command imports a Blockchain configuration from a running network.

By default, an imported Blockchain
doesn't overwrite an existing Blockchain with the same name. To allow overwrites, provide the --force
flag.`,
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.DefaultSupportedNetworkOptions)

	cmd.Flags().StringVar(&nodeURL, "node-url", "", "[optional] URL of an already running validator")

	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "import a subnet-evm")
	cmd.Flags().BoolVar(&useCustomVM, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVar(
		&overwriteImport,
		"force",
		false,
		"overwrite the existing configuration if one exists",
	)
	cmd.Flags().StringVar(
		&blockchainIDStr,
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
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	var reply *info.GetNodeVersionReply

	if nodeURL == "" {
		yes, err := app.Prompt.CaptureNoYes("Have validator nodes already been deployed to this blockchain?")
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
	if blockchainIDStr == "" {
		blockchainID, err = app.Prompt.CaptureID("What is the ID of the blockchain?")
		if err != nil {
			return err
		}
	} else {
		blockchainID, err = ids.FromString(blockchainIDStr)
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
	blockchainName := createChainTx.ChainName
	genBytes := createChainTx.GenesisData

	ux.Logger.PrintToUser("Retrieved information. BlockchainID: %s, SubnetID: %s, Name: %s, VMID: %s",
		blockchainID.String(),
		subnetID.String(),
		blockchainName,
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
		Name: blockchainName,
		VM:   vmType,
		Networks: map[string]models.NetworkData{
			network.Name(): {
				SubnetID:     subnetID,
				BlockchainID: blockchainID,
			},
		},
		Subnet:       blockchainName,
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
			versions, err = app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName, "", application.All)
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

	if err = app.WriteGenesisFile(blockchainName, genBytes); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Blockchain %q imported successfully", sc.Name)

	return nil
}

func importL1(blockchainIDStr string, rpcURL string, network models.Network) (models.Sidecar, error) {
	var sc models.Sidecar

	blockchainID, err := precompiles.WarpPrecompileGetBlockchainID(rpcURL)
	if err != nil {
		if blockchainIDStr == "" {
			blockchainID, err = app.Prompt.CaptureID("What is the Blockchain ID?")
			if err != nil {
				return models.Sidecar{}, err
			}
		} else {
			blockchainID, err = ids.FromString(blockchainIDStr)
			if err != nil {
				return models.Sidecar{}, err
			}
		}
	}
	subnetID, err := blockchain.GetSubnetIDFromBlockchainID(blockchainID, network)
	if err != nil {
		return models.Sidecar{}, err
	}

	subnetInfo, err := blockchain.GetSubnet(subnetID, network)
	if err != nil {
		return models.Sidecar{}, err
	}
	if subnetInfo.IsPermissioned {
		return models.Sidecar{}, fmt.Errorf("unable to import non sovereign Subnets")
	}
	validatorManagerAddress = "0x" + hex.EncodeToString(subnetInfo.ManagerAddress)

	// add validator without blockchain arg is only for l1s
	sc = models.Sidecar{
		Sovereign: true,
	}

	sc.ValidatorManagement, err = validatorManagerSDK.GetValidatorManagerType(rpcURL, common.HexToAddress(validatorManagerAddress))
	if err != nil {
		return models.Sidecar{}, fmt.Errorf("could not obtain validator manager type: %w", err)
	}

	if sc.ValidatorManagement == validatormanagertypes.ProofOfAuthority {
		owner, err := contract.GetContractOwner(rpcURL, common.HexToAddress(validatorManagerAddress))
		if err != nil {
			return models.Sidecar{}, err
		}
		sc.ValidatorManagerOwner = owner.String()
	}

	sc.Networks = make(map[string]models.NetworkData)

	sc.Networks[network.Name()] = models.NetworkData{
		SubnetID:                subnetID,
		BlockchainID:            blockchainID,
		ValidatorManagerAddress: validatorManagerAddress,
		RPCEndpoints:            []string{rpcURL},
	}
	// TODO: we are currently assuming that all remote L1s are ACP99
	// we should drop support for non acp 99 L1s, and once that's done remove the line below
	sc.UseACP99 = true
	return sc, err
}

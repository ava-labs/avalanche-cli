// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	subnetIDstr     string
	useSubnetEvm    bool
	useCustomVM     bool
	rpcURL          string
	noRPCAvailable  bool
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

	cmd.Flags().StringVar(&nodeEndpoint, "node-endpoint", "", "[optional] URL of an already running validator")

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
	cmd.Flags().StringVar(&rpcURL, "rpc", "", "rpc endpoint for the blockchain")
	cmd.Flags().BoolVar(&noRPCAvailable, "no-rpc-available", false, "use this when an RPC if offline and can't be accesed")
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

	var blockchainID ids.ID
	if blockchainIDStr != "" {
		blockchainID, err = ids.FromString(blockchainIDStr)
		if err != nil {
			return err
		}
	}

	sc, genBytes, err := importBlockchain(network, rpcURL, !noRPCAvailable, blockchainID, ux.Logger.PrintToUser)
	if err != nil {
		return err
	}

	sc.TokenName = constants.DefaultTokenName
	sc.TokenSymbol = constants.DefaultTokenSymbol

	sc.VM, err = vm.PromptVMType(app, useSubnetEvm, useCustomVM)
	if err != nil {
		return err
	}

	var nodeVersionReply *info.GetNodeVersionReply
	if nodeEndpoint == "" {
		yes, err := app.Prompt.CaptureNoYes("Have validator nodes already been deployed to this blockchain?")
		if err != nil {
			return err
		}
		if yes {
			nodeEndpoint, err = app.Prompt.CaptureString(
				"Please provide an API URL of such a node so we can query its VM version (e.g. http://111.22.33.44:5555)")
			if err != nil {
				return err
			}
			ctx, cancel := utils.GetAPIContext()
			defer cancel()
			infoAPI := info.NewClient(nodeEndpoint)
			options := []rpc.Option{}
			nodeVersionReply, err = infoAPI.GetNodeVersion(ctx, options...)
			if err != nil {
				return fmt.Errorf("failed to query node - is it running and reachable? %w", err)
			}
		}
	}

	var versions []string
	if nodeVersionReply != nil {
		// a node was queried
		for _, v := range nodeVersionReply.VMVersions {
			if v == sc.ImportedVMID {
				sc.VMVersion = v
				break
			}
		}
		sc.RPCVersion = int(nodeVersionReply.RPCProtocolVersion)
	} else if sc.VM == models.SubnetEvm {
		// no node was queried, ask the user
		versions, err = app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName, "", application.All)
		if err != nil {
			return err
		}
		sc.VMVersion, err = app.Prompt.CaptureList("Pick the version for this VM", versions)
		if err != nil {
			return err
		}
		sc.RPCVersion, err = vm.GetRPCProtocolVersion(app, sc.VM, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed getting RPCVersion for VM type %s with version %s", sc.VM, sc.VMVersion)
		}
	}

	if sc.VM == models.SubnetEvm {
		var genesis core.Genesis
		if err := json.Unmarshal(genBytes, &genesis); err != nil {
			return err
		}
		sc.ChainID = genesis.Config.ChainID.String()
	}

	if err := app.CreateSidecar(&sc); err != nil {
		return fmt.Errorf("failed creating the sidecar for import: %w", err)
	}

	if err = app.WriteGenesisFile(sc.Name, genBytes); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Blockchain %q imported successfully", sc.Name)

	return nil
}

func importBlockchain(
	network models.Network,
	rpcURL string,
	rpcIsAvailable bool,
	blockchainID ids.ID,
	printFunc func(msg string, args ...interface{}),
) (models.Sidecar, []byte, error) {
	var err error

	if !rpcIsAvailable && rpcURL != "" {
		return models.Sidecar{}, nil, fmt.Errorf("RPC can't be both non empty and unavailable")
	}

	if rpcIsAvailable && rpcURL == "" {
		rpcURL, err = app.Prompt.CaptureURL("What is the RPC endpoint?", false)
		if err != nil {
			return models.Sidecar{}, nil, err
		}
	}

	if blockchainID == ids.Empty {
		var err error
		if rpcIsAvailable {
			blockchainID, _ = precompiles.WarpPrecompileGetBlockchainID(rpcURL)
		}
		if blockchainID == ids.Empty {
			blockchainID, err = app.Prompt.CaptureID("What is the Blockchain ID?")
			if err != nil {
				return models.Sidecar{}, nil, err
			}
		}
	}

	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
	if err != nil {
		return models.Sidecar{}, nil, err
	}

	subnetID := createChainTx.SubnetID
	vmID := createChainTx.VMID
	blockchainName := createChainTx.ChainName
	genBytes := createChainTx.GenesisData

	printFunc("Retrieved information:")
	printFunc("  Name: %s", blockchainName)
	printFunc("  BlockchainID: %s", blockchainID.String())
	printFunc("  SubnetID: %s", subnetID.String())
	printFunc("  VMID: %s", vmID.String())

	subnetInfo, err := blockchain.GetSubnet(subnetID, network)
	if err != nil {
		return models.Sidecar{}, nil, err
	}

	sc := models.Sidecar{
		Name: blockchainName,
		Networks: map[string]models.NetworkData{
			network.Name(): {
				SubnetID:     subnetID,
				BlockchainID: blockchainID,
			},
		},
		Subnet:          blockchainName,
		Version:         constants.SidecarVersion,
		ImportedVMID:    vmID.String(),
		ImportedFromAPM: true,
	}

	if rpcIsAvailable {
		e := sc.Networks[network.Name()]
		e.RPCEndpoints = []string{rpcURL}
		sc.Networks[network.Name()] = e
	}

	if !subnetInfo.IsPermissioned {
		sc.Sovereign = true
		validatorManagerAddress = "0x" + hex.EncodeToString(subnetInfo.ManagerAddress)
		e := sc.Networks[network.Name()]
		e.ValidatorManagerAddress = validatorManagerAddress
		sc.Networks[network.Name()] = e
		printFunc("  Validator Manager Address: %s", validatorManagerAddress)
		if rpcIsAvailable {
			sc.ValidatorManagement, err = validatorManagerSDK.GetValidatorManagerType(rpcURL, common.HexToAddress(validatorManagerAddress))
			if err != nil {
				return models.Sidecar{}, nil, fmt.Errorf("could not obtain validator manager type: %w", err)
			}
			printFunc("  Validation Kind: %s", sc.ValidatorManagement)
			if sc.ValidatorManagement == validatormanagertypes.ProofOfAuthority {
				owner, err := contract.GetContractOwner(rpcURL, common.HexToAddress(validatorManagerAddress))
				if err != nil {
					return models.Sidecar{}, nil, err
				}
				sc.ValidatorManagerOwner = owner.String()
				printFunc("  Validator Manager Owner: %s", sc.ValidatorManagerOwner)
			}
		}
	}

	return sc, genBytes, err
}

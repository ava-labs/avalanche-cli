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
	contract "github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	contractsdk "github.com/ava-labs/avalanche-cli/sdk/evm/contract"
	"github.com/ava-labs/avalanche-cli/sdk/evm/precompiles"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/ids"
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

	sc, genBytes, err := importBlockchain(network, rpcURL, blockchainID, ux.Logger.PrintToUser)
	if err != nil {
		return err
	}

	sc.TokenName = constants.DefaultTokenName
	sc.TokenSymbol = constants.DefaultTokenSymbol

	sc.VM, err = vm.PromptVMType(app, useSubnetEvm, useCustomVM)
	if err != nil {
		return err
	}

	if sc.VM == models.SubnetEvm {
		versions, err := app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName, "", application.All)
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
	blockchainID ids.ID,
	printFunc func(msg string, args ...interface{}),
) (models.Sidecar, []byte, error) {
	var err error

	if rpcURL == "" {
		rpcURL, err = app.Prompt.CaptureStringAllowEmpty("What is the RPC endpoint?")
		if err != nil {
			return models.Sidecar{}, nil, err
		}
		if rpcURL != "" {
			if err := prompts.ValidateURLFormat(rpcURL); err != nil {
				return models.Sidecar{}, nil, fmt.Errorf("invalid url format: %w", err)
			}
		}
	}

	if blockchainID == ids.Empty {
		var err error
		if rpcURL != "" {
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
	if subnetInfo.IsPermissioned {
		printFunc("  Blockchain is Not Sovereign")
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

	if rpcURL != "" {
		e := sc.Networks[network.Name()]
		e.RPCEndpoints = []string{rpcURL}
		sc.Networks[network.Name()] = e
	}

	if !subnetInfo.IsPermissioned {
		sc.Sovereign = true
		sc.UseACP99 = true
		validatorManagerAddress := "0x" + hex.EncodeToString(subnetInfo.ManagerAddress)
		validatorManagerRPCEndpoint := ""
		validatorManagerBlockchainID := subnetInfo.ManagerChainID
		printFunc("  Validator Manager Address: %s", validatorManagerAddress)
		printFunc("  Validator Manager BlockchainID: %s", subnetInfo.ManagerChainID)
		if blockchainID == subnetInfo.ManagerChainID {
			// manager lives at L1
			validatorManagerRPCEndpoint = rpcURL
		} else {
			cChainID, err := contract.GetBlockchainID(app, network, contract.ChainSpec{CChain: true})
			if err != nil {
				return models.Sidecar{}, nil, fmt.Errorf("could not get C-Chain ID for %s: %w", network.Name(), err)
			}
			if cChainID == subnetInfo.ManagerChainID {
				// manager lives at C-Chain
				validatorManagerRPCEndpoint = network.CChainEndpoint()
			} else {
				printFunc("The Validator Manager is not deployed on L1 or on C-Chain")
				validatorManagerRPCEndpoint, err = app.Prompt.CaptureURL("What is the Validator Manager RPC endpoint?", false)
				if err != nil {
					return models.Sidecar{}, nil, err
				}
			}
		}
		printFunc("  Validator Manager RPC: %s", validatorManagerRPCEndpoint)
		e := sc.Networks[network.Name()]
		e.ValidatorManagerAddress = validatorManagerAddress
		e.ValidatorManagerBlockchainID = validatorManagerBlockchainID
		e.ValidatorManagerRPCEndpoint = validatorManagerRPCEndpoint
		sc.Networks[network.Name()] = e
		if validatorManagerRPCEndpoint != "" {
			validatorManagement, ownerAddress, specializedValidatorManagerAddress, err := GetBaseValidatorManagerInfo(
				validatorManagerRPCEndpoint,
				common.HexToAddress(validatorManagerAddress),
			)
			if err != nil {
				return models.Sidecar{}, nil, err
			}
			sc.ValidatorManagement = validatorManagement
			if sc.ValidatorManagement == validatormanagertypes.ProofOfAuthority {
				sc.ValidatorManagerOwner = ownerAddress.String()
			}
			if specializedValidatorManagerAddress != (common.Address{}) {
				printFunc("  Specialized Validator Manager Address: %s", specializedValidatorManagerAddress)
				e := sc.Networks[network.Name()]
				e.ValidatorManagerAddress = specializedValidatorManagerAddress.String()
				sc.Networks[network.Name()] = e
			}
			printFunc("  Validation Kind: %s", sc.ValidatorManagement)
			if sc.ValidatorManagement == validatormanagertypes.ProofOfAuthority {
				printFunc("  Validator Manager Owner: %s", sc.ValidatorManagerOwner)
			}
		}
	}

	return sc, genBytes, err
}

// returns validator manager type, owner if it is PoA, specialized address if it has specialization, error
func GetBaseValidatorManagerInfo(
	validatorManagerRPCEndpoint string,
	validatorManagerAddress common.Address,
) (validatormanagertypes.ValidatorManagementType, common.Address, common.Address, error) {
	validatorManagement := validatorManagerSDK.GetValidatorManagerType(validatorManagerRPCEndpoint, validatorManagerAddress)
	if validatorManagement == validatormanagertypes.UndefinedValidatorManagement {
		return validatorManagement, common.Address{}, common.Address{}, fmt.Errorf("could not infer validator manager type")
	}
	if validatorManagement == validatormanagertypes.ProofOfAuthority {
		// a v2.0.0 validator manager can be identified as PoA for two cases:
		// - it is PoA
		// - it is a validator manager used by v2.0.0 PoS or another specialized validator manager,
		//   in which case the main manager interacts with the P-Chain, and the specialized manager, which is the
		//   owner of this main manager, interacts with the users
		owner, err := contractsdk.GetContractOwner(validatorManagerRPCEndpoint, validatorManagerAddress)
		if err != nil {
			return validatorManagement, common.Address{}, common.Address{}, err
		}
		// check if the owner is a specialized PoS validator manager
		// if this is the case, GetValidatorManagerType will return the corresponding type
		specializedValidatorManagement := validatorManagerSDK.GetValidatorManagerType(validatorManagerRPCEndpoint, owner)
		if specializedValidatorManagement != validatormanagertypes.UndefinedValidatorManagement {
			return specializedValidatorManagement, common.Address{}, owner, nil
		} else {
			return validatorManagement, owner, common.Address{}, nil
		}
	}
	return validatorManagement, common.Address{}, common.Address{}, nil
}

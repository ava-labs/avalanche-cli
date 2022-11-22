// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/rpc"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/core"
	"github.com/spf13/cobra"
)

var (
	genesisFilePath string
	subnetIDstr     string
	nodeURL         string
)

// avalanche subnet import
func newImportFromNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import-running [subnetPath]",
		Short:        "Import an existing subnet config from running subnets",
		RunE:         importRunningSubnet,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `The subnet import command will import a subnet configuration from a running network.

The genesis file should be available from the disk for this to work. 
By default, an imported subnet will not overwrite an existing subnet with the same name. 
To allow overwrites, provide the --force flag.`,
	}

	cmd.Flags().StringVar(&nodeURL, "node-url", "", "[optional] URL of an already running subnet validator")

	cmd.Flags().BoolVar(&deployTestnet, "fuji", false, "import from `fuji` (alias for `testnet`)")
	cmd.Flags().BoolVar(&deployTestnet, "testnet", false, "import from `testnet` (alias for `fuji`)")
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "import from `mainnet`")
	cmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "import a subnet-evm")
	cmd.Flags().BoolVar(&useSpacesVM, "spacesvm", false, "use the SpacesVM as the base template")
	cmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	cmd.Flags().BoolVarP(
		&overwriteImport,
		"force",
		"f",
		false,
		"overwrite the existing configuration if one exists",
	)
	cmd.Flags().StringVar(
		&genesisFilePath,
		"genesis-file-path",
		"",
		"path to the genesis file",
	)
	cmd.Flags().StringVar(
		&subnetIDstr,
		"subnet-id",
		"",
		"the subnet ID",
	)
	return cmd
}

func importRunningSubnet(cmd *cobra.Command, args []string) error {
	var err error

	var network models.Network
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}

	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose a network to import from",
			[]string{models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	if genesisFilePath == "" {
		genesisFilePath, err = app.Prompt.CaptureExistingFilepath("Provide the path to the genesis file")
		if err != nil {
			return err
		}
	}

	var reply *info.GetNodeVersionReply

	if nodeURL == "" {
		yes, err := app.Prompt.CaptureYesNo("Have nodes already been deployed to this subnet?")
		if err != nil {
			return err
		}
		if yes {
			nodeURL, err = app.Prompt.CaptureString(
				"Please provide an API URL of such a node so we can query its VM version (e.g. 111.22.33.44:5555)")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
			defer cancel()
			infoAPI := info.NewClient(nodeURL)
			options := []rpc.Option{}
			reply, err = infoAPI.GetNodeVersion(ctx, options...)
			if err != nil {
				return fmt.Errorf("failed to query node - is it running and reachable? %w", err)
			}
		}
	}

	var subnetID ids.ID
	if subnetIDstr == "" {
		subnetID, err = app.Prompt.CaptureID("What is the ID of the subnet?")
		if err != nil {
			return err
		}
	} else {
		subnetID, err = ids.FromString(subnetIDstr)
		if err != nil {
			return err
		}
	}

	var pubAPI string
	switch network {
	case models.Fuji:
		pubAPI = constants.FujiAPIEndpoint
	case models.Mainnet:
		pubAPI = constants.MainnetAPIEndpoint
	}
	client := platformvm.NewClient(pubAPI)
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	defer cancel()
	options := []rpc.Option{}

	ux.Logger.PrintToUser("Getting information from the %s network...", network.String())

	chains, err := client.GetBlockchains(ctx, options...)
	if err != nil {
		return err
	}

	var (
		blockchainID, vmID ids.ID
		subnetName         string
	)

	for _, ch := range chains {
		// NOTE: This supports only one chain per subnet
		if ch.SubnetID == subnetID {
			blockchainID = ch.ID
			vmID = ch.VMID
			subnetName = ch.Name
			break
		}
	}

	if blockchainID == ids.Empty || vmID == ids.Empty {
		return fmt.Errorf("subnet ID %s not found on this network", subnetIDstr)
	}

	ux.Logger.PrintToUser("Retrieved information. BlockchainID: %s, Name: %s, VMID: %s",
		blockchainID.String(),
		subnetName,
		vmID.String(),
	)
	// TODO: it's probably possible to deploy VMs with the same name on a public network
	// In this case, an import could clash because the tool supports unique names only

	genBytes, err := os.ReadFile(genesisFilePath)
	if err != nil {
		return err
	}

	if err = app.WriteGenesisFile(subnetName, genBytes); err != nil {
		return err
	}

	vmType := getVMFromFlag()
	if vmType == "" {
		subnetTypeStr, err := app.Prompt.CaptureList(
			"What's this VM's type?",
			[]string{models.SubnetEvm, models.SpacesVM, models.CustomVM},
		)
		if err != nil {
			return err
		}
		vmType = models.VMTypeFromString(subnetTypeStr)
	}

	sc := &models.Sidecar{
		Name:            subnetName,
		VM:              vmType,
		ImportedFromAPM: false,
		Networks: map[string]models.NetworkData{
			network.String(): {
				SubnetID:     subnetID,
				BlockchainID: blockchainID,
			},
		},
		Subnet:  subnetName,
		Version: constants.SidecarVersion,
	}

	var versions []string
	switch vmType {
	case models.SubnetEvm:
		versions, err = app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName)
		if err != nil {
			return err
		}
		sc.VMVersion, err = app.Prompt.CaptureList("Pick the version for this VM", versions)
	case models.SpacesVM:
		versions, err = app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SpacesVMRepoName)
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

	// hasn't been set in reply
	if sc.RPCVersion == 0 {
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
		/*
			  // TODO: can we get this one?
				TokenName       string
		*/
	}

	vmIDstr := vmID.String()
	sc.ImportedVMID = vmIDstr // TODO: Is this correct?
	if reply != nil {
		for _, v := range reply.VMVersions {
			if v == vmIDstr {
				sc.VMVersion = v
			}
		}
		sc.RPCVersion = int(reply.RPCProtocolVersion)
	}

	if err := app.CreateSidecar(sc); err != nil {
		return fmt.Errorf("failed creating the sidecar for import: %w", err)
	}

	ux.Logger.PrintToUser("Subnet %s imported successfully", sc.Name)

	return nil
}

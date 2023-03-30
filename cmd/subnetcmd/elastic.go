// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/localnetworkinterface"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanchego/genesis"

	es "github.com/ava-labs/avalanche-cli/pkg/elasticsubnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	localDeployment   = "Existing local deployment"
	fujiDeployment    = "Fuji (coming soon)"
	mainnetDeployment = "Mainnet (coming soon)"
)

// avalanche subnet elastic
func newElasticCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "elastic [subnetName]",
		Short: "Transforms a subnet into elastic subnet",
		Long: `The elastic command enables anyone to be a validator of a Subnet by simply staking its token on the 
P-Chain. When enabling Elastic Validation, the creator permanently locks the Subnet from future modification 
(they relinquish their control keys), specifies an Avalanche Native Token (ANT) that validators must use for staking 
and that will be distributed as staking rewards, and provides a set of parameters that govern how the Subnet’s staking 
mechanics will work.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         elasticSubnetConfig,
	}
	return cmd
}

func checkIfSubnetIsElasticOnLocal(sc models.Sidecar) bool {
	if _, ok := sc.ElasticSubnet[models.Local.String()]; ok {
		return true
	}
	return false
}

func getVMBin(sc models.Sidecar, args []string) (string, error) {
	chains, err := validateSubnetNameAndGetChains(args)
	if err != nil {
		return "", err
	}
	chain := chains[0]
	switch sc.VM {
	case models.SubnetEvm:
		vmBin, err := binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return "", fmt.Errorf("failed to install subnet-evm: %w", err)
		}
		return vmBin, nil
	case models.SpacesVM:
		vmBin, err := binutils.SetupSpacesVM(app, sc.VMVersion)
		if err != nil {
			return "", fmt.Errorf("failed to install spacesvm: %w", err)
		}
		return vmBin, nil
	case models.CustomVM:
		vmBin := binutils.SetupCustomBin(app, chain)
		return vmBin, nil
	default:
		return "", fmt.Errorf("unknown vm: %s", sc.VM)
	}
}
func elasticSubnetConfig(_ *cobra.Command, args []string) error {
	cancel := make(chan struct{})
	defer close(cancel)
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	networkToUpgrade, err := selectNetworkToTransform(sc, []string{})
	if err != nil {
		return err
	}

	switch networkToUpgrade {
	case localDeployment:
	case fujiDeployment:
		return errors.New("elastic subnet transformation is not yet supported on Fuji network")
	case mainnetDeployment:
		return errors.New("elastic subnet transformation is not yet supported on Mainnet")
	}

	if checkIfSubnetIsElasticOnLocal(sc) {
		return fmt.Errorf(fmt.Sprintf("%s is already an elastic subnet", subnetName))
	}

	yes, err := app.Prompt.CaptureNoYes("WARNING: Transforming a Permissioned Subnet into an Elastic Subnet is an irreversible operation. Continue?")
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	tokenName, err := getTokenName()
	if err != nil {
		return err
	}
	tokenSymbol, err := getTokenSymbol()
	if err != nil {
		return err
	}
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	elasticSubnetConfig, err := es.GetElasticSubnetConfig(app, tokenSymbol)
	if err != nil {
		return err
	}
	vmBin, err := getVMBin(sc, args)
	if err != nil {
		return err
	}
	// skip rpc check if using custom vm
	if sc.VM != models.CustomVM {
		// check if selected version matches what is currently running
		nc := localnetworkinterface.NewStatusChecker()
		userProvidedAvagoVersion, err = checkForInvalidDeployAndGetAvagoVersion(nc, sc.RPCVersion)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("Starting Elastic Subnet Transformation")
	go ux.PrintWait(cancel)
	for network := range sc.Networks {
		if network == models.Local.String() {
			subnetID := sc.Networks[network].SubnetID
			elasticSubnetConfig.SubnetID = subnetID
			if subnetID == ids.Empty {
				return errNoSubnetID
			}

			deployer := subnet.NewLocalDeployer(app, userProvidedAvagoVersion, vmBin)
			txID, assetID, err := deployer.IssueTransformSubnetTx(elasticSubnetConfig, keyChain, subnetID, tokenName, tokenSymbol, elasticSubnetConfig.MaxSupply)
			if err != nil {
				return err
			}
			elasticSubnetConfig.AssetID = assetID
			PrintTransformResults(subnetName, txID, subnetID, tokenName, tokenSymbol, assetID)
			if err = app.CreateElasticSubnetConfig(subnetName, &elasticSubnetConfig); err != nil {
				return err
			}
			if err = app.UpdateSidecarElasticSubnet(&sc, models.Local, subnetID, assetID, txID, tokenName, tokenSymbol); err != nil {
				return err
			}
		}
	}

	return nil
}

// select which network to transform to elastic subnet
func selectNetworkToTransform(sc models.Sidecar, networkOptions []string) (string, error) {
	networkPrompt := "Which network should transform into an elastic Subnet?"

	// get locally deployed subnets from file since network is shut down
	locallyDeployedSubnets, err := subnet.GetLocallyDeployedSubnetsFromFile(app)
	if err != nil {
		return "", fmt.Errorf("unable to read deployed subnets: %w", err)
	}

	for _, subnet := range locallyDeployedSubnets {
		if subnet == sc.Name {
			networkOptions = append(networkOptions, localDeployment)
		}
	}

	// check if subnet deployed on fuji
	if _, ok := sc.Networks[models.Fuji.String()]; ok {
		networkOptions = append(networkOptions, fujiDeployment)
	}

	// check if subnet deployed on mainnet
	if _, ok := sc.Networks[models.Mainnet.String()]; ok {
		networkOptions = append(networkOptions, mainnetDeployment)
	}

	if len(networkOptions) == 0 {
		return "", errors.New("no deployment target available, please first deploy created subnet")
	}

	selectedDeployment, err := app.Prompt.CaptureList(networkPrompt, networkOptions)
	if err != nil {
		return "", err
	}
	return selectedDeployment, nil
}

func PrintTransformResults(chain string, txID ids.ID, subnetID ids.ID, tokenName string, tokenSymbol string, assetID ids.ID) {
	const art = "\n  ______ _           _   _         _____       _                _     _______                   __                        _____                              __       _ " +
		"\n |  ____| |         | | (_)       / ____|     | |              | |   |__   __|                 / _|                      / ____|                            / _|     | |" +
		"\n | |__  | | __ _ ___| |_ _  ___  | (___  _   _| |__  _ __   ___| |_     | |_ __ __ _ _ __  ___| |_ ___  _ __ _ __ ___   | (___  _   _  ___ ___ ___  ___ ___| |_ _   _| |" +
		"\n |  __| | |/ _` / __| __| |/ __|  \\___ \\| | | | '_ \\| '_ \\ / _ \\ __|    | | '__/ _` | '_ \\/ __|  _/ _ \\| '__| '_ ` _ \\   \\___ \\| | | |/ __/ __/ _ \\/ __/ __|  _| | | | |" +
		"\n | |____| | (_| \\__ \\ |_| | (__   ____) | |_| | |_) | | | |  __/ |_     | | | | (_| | | | \\__ \\ || (_) | |  | | | | | |  ____) | |_| | (_| (_|  __/\\__ \\__ \\ | | |_| | |" +
		"\n |______|_|\\__,_|___/\\__|_|\\___| |_____/ \\__,_|_.__/|_| |_|\\___|\\__|    |_|_|  \\__,_|_| |_|___/_| \\___/|_|  |_| |_| |_| |_____/ \\__,_|\\___\\___\\___||___/___/_|  \\__,_|_|" +
		"\n"
	fmt.Print(art)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Token Name", tokenName})
	table.Append([]string{"Token Symbol", tokenSymbol})
	table.Append([]string{"Asset ID", assetID.String()})
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"P-Chain TXID", txID.String()})
	table.Render()
}

func getTokenName() (string, error) {
	ux.Logger.PrintToUser("Select a name for your subnet's native token")
	tokenName, err := app.Prompt.CaptureString("Token name")
	if err != nil {
		return "", err
	}
	return tokenName, nil
}

func getTokenSymbol() (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenSymbol, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}
	return tokenSymbol, nil
}

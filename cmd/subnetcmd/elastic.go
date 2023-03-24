// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/genesis"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	defaultInitialSupply            = 240_000_000
	defaultMaximumSupply            = 720_000_000
	defaultMinConsumptionRate       = 0.1
	defaultMaxConsumptionRate       = 0.12
	defaultMintingPeriod            = 365 * 24 * time.Hour
	defaultMinValidatorStake        = 2_000
	defaultMaxValidatorStake        = 3_000_000
	defaultMinStakeDuration         = 14 * 24 * time.Hour
	defaultMaxStakeDuration         = 365 * 24 * time.Hour
	defaultMinDelegationFee         = 20_000
	defaultMinDelegatorStake        = 25
	defaultMaxValidatorWeightFactor = 5
	defaultUptimeRequirement        = 0.8

	localDeployment   = "Existing local deployment"
	fujiDeployment    = "Fuji (coming soon)"
	mainnetDeployment = "Mainnet (coming soon)"
)

// avalanche subnet create
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
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}
func getDefaultElasticSubnetConfig() (models.ElasticSubnetConfig, error) {
	const (
		defaultConfig = "Use default elastic subnet config"
	)
	elasticSubnetConfigOptions := []string{defaultConfig}

	chosenConfig, err := app.Prompt.CaptureList(
		"How would you like to set fees",
		elasticSubnetConfigOptions,
	)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	elasticSubnetConfig := models.ElasticSubnetConfig{
		InitialSupply:            defaultInitialSupply,
		MaxSupply:                defaultMaximumSupply,
		MinConsumptionRate:       defaultMinConsumptionRate * reward.PercentDenominator,
		MaxConsumptionRate:       defaultMaxConsumptionRate * reward.PercentDenominator,
		MinValidatorStake:        defaultMinValidatorStake,
		MaxValidatorStake:        defaultMaxValidatorStake,
		MinStakeDuration:         defaultMinStakeDuration,
		MaxStakeDuration:         defaultMaxStakeDuration,
		MinDelegationFee:         defaultMinDelegationFee,
		MinDelegatorStake:        defaultMinDelegatorStake,
		MaxValidatorWeightFactor: defaultMaxValidatorWeightFactor,
		UptimeRequirement:        defaultUptimeRequirement * reward.PercentDenominator,
	}
	if chosenConfig == defaultConfig {
		return elasticSubnetConfig, nil
	}

	return models.ElasticSubnetConfig{}, nil
}
func elasticSubnetConfig(_ *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	yes, err := app.Prompt.CaptureNoYes("WARNING: Transforming a Permissioned Subnet into an Elastic Subnet is an irreversible operation. Continue?")
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	networkToUpgrade, err := selectNetworkToTransform(sc, []string{})
	if err != nil {
		return err
	}

	switch networkToUpgrade {
	case localDeployment:
	case fujiDeployment:
		return errors.New("Elastic subnet transformation is not yet supported on Fuji network")
	case mainnetDeployment:
		return errors.New("Elastic subnet transformation is not yet supported on Mainnet")
	}

	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	elasticSubnetConfig, err := getDefaultElasticSubnetConfig()
	if err != nil {
		return err
	}
	for network, _ := range sc.Networks {
		if network == models.Local.String() {
			subnetID := sc.Networks[network].SubnetID
			elasticSubnetConfig.SubnetID = subnetID
			if subnetID == ids.Empty {
				return errNoSubnetID
			}
			deployer := subnet.NewPublicDeployer(app, false, keyChain, models.Local)
			txID, assetID, err := deployer.IssueTransformSubnetTx(elasticSubnetConfig, subnetID)
			if err != nil {
				return err
			}
			elasticSubnetConfig.AssetID = assetID

			PrintTransformResults(subnetName, txID, subnetID)
			if err = app.CreateElasticSubnetConfig(subnetName, &elasticSubnetConfig); err != nil {
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
		return "", errors.New("no deployment target available")
	}

	selectedDeployment, err := app.Prompt.CaptureList(networkPrompt, networkOptions)
	if err != nil {
		return "", err
	}
	return selectedDeployment, nil
}

func PrintTransformResults(chain string, txID ids.ID, subnetID ids.ID) {
	header := []string{"Subnet Elastic Transform Results", ""}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)
	table.Append([]string{"Chain Name", chain})
	table.Append([]string{"Subnet ID", subnetID.String()})
	table.Append([]string{"P-Chain TXID", txID.String()})
	table.Render()
}

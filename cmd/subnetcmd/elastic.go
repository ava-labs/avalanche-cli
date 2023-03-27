// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/genesis"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	prompts "github.com/ava-labs/avalanche-cli/pkg/prompts"
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

func getElasticSubnetConfig() (models.ElasticSubnetConfig, error) {
	const (
		defaultConfig   = "Use default elastic subnet config"
		customizeConfig = "Customize elastic subnet config"
	)
	elasticSubnetConfigOptions := []string{defaultConfig, customizeConfig}

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
	} else if chosenConfig == customizeConfig {

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
		return errors.New("elastic subnet transformation is not yet supported on Fuji network")
	case mainnetDeployment:
		return errors.New("elastic subnet transformation is not yet supported on Mainnet")
	}
	getCustomElasticSubnetConfig()
	testKey := genesis.EWOQKey
	keyChain := secp256k1fx.NewKeychain(testKey)
	elasticSubnetConfig, err := getElasticSubnetConfig()
	if err != nil {
		return err
	}
	for network := range sc.Networks {
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
func getCustomElasticSubnetConfig() (models.ElasticSubnetConfig, error) {
	tokenName, err := getTokenName()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	initialSupply, err := getInitialSupply(tokenName)
	fmt.Printf("initial supply amount %s \n", initialSupply)
	maxSupply, err := getMaximumSupply(tokenName, initialSupply)
	fmt.Printf("max supply amount %s \n", maxSupply)
	minConsumptionRate, maxConsumptionRate, err := getConsumptionRate()
	fmt.Printf("consumption rate %s  %s\n", minConsumptionRate, maxConsumptionRate)

	return models.ElasticSubnetConfig{}, err
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

func getTokenName() (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}
	return tokenName, nil
}

func getInitialSupply(tokenName string) (uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Initial Supply of %s", tokenName))
	initialSupply, err := app.Prompt.CaptureUint64("Initial Supply amount")
	if err != nil {
		return 0, err
	}
	return initialSupply, nil
}

func getMaximumSupply(tokenName string, initialSupply uint64) (uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Supply of %s", tokenName))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThanEq
	comparator.CompareValue = initialSupply
	comparatorMap["Initial Supply"] = comparator
	maxSupply, err := app.Prompt.CaptureUint64Compare("Maximum Supply amount", comparatorMap)

	if err != nil {
		return 0, err
	}
	return maxSupply, nil
}

func getConsumptionRate() (uint64, uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Minimum Consumption Rate"))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = reward.PercentDenominator
	comparatorMap["Percent Denominator(1_0000_0000)"] = comparator
	minConsumptionRate, err := app.Prompt.CaptureUint64Compare("Minimum Consumption Rate", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Consumption Rate"))
	comparator.CompareType = prompts.MoreThanEq
	comparator.CompareValue = minConsumptionRate
	comparatorMap["Mininum Consumption Rate"] = comparator
	maxConsumptionRate, err := app.Prompt.CaptureUint64Compare("Maximum Consumption Rate", comparatorMap)
	if err != nil {
		return 0, 0, err
	}
	return minConsumptionRate, maxConsumptionRate, nil
}

func getValidatorStake(initialSupply uint64, maximumSupply uint64) (uint64, uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Minimum Validator Stake"))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = 0
	comparatorMap["0"] = comparator
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = initialSupply
	comparatorMap["Initial Supply"] = comparator
	minValidatorStake, err := app.Prompt.CaptureUint64Compare("Minimum Validator Stake", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Validator Stake"))
	comparatorMap = map[string]prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = minValidatorStake
	comparatorMap["Minimum Validator Stake"] = comparator
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = maximumSupply
	comparatorMap["Maximum Supply"] = comparator
	maxValidatorStake, err := app.Prompt.CaptureUint64Compare("Maximum Validator Stake", comparatorMap)
	if err != nil {
		return 0, 0, err
	}
	return minValidatorStake, maxValidatorStake, nil
}

func getStakeDuration(tokenName string, initialSupply uint64) (uint64, uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Minimum Stake Duration"))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = 0
	comparatorMap["0"] = comparator
	minStakeDuration, err := app.Prompt.CaptureUint64Compare("Minimum Stake Duration", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Stake Duration"))
	comparatorMap = map[string]prompts.Comparator{}
	comparator = prompts.Comparator{}
	comparator.CompareType = prompts.MoreThanEq
	comparator.CompareValue = minStakeDuration
	comparatorMap["Minimum Stake Duration"] = comparator
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = uint64(defaultMaxStakeDuration)
	comparatorMap["Global Max Stake Duration"] = comparator
	maxStakeDuration, err := app.Prompt.CaptureUint64Compare("Maximum Stake Duration", comparatorMap)
	if err != nil {
		return 0, 0, err
	}
	return minStakeDuration, maxStakeDuration, nil
}

func getMinDelegationFee() (uint32, error) {
	ux.Logger.PrintToUser("Select the Minimum Delegation Fee")
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = reward.PercentDenominator
	comparatorMap["Percent Denominator(1_0000_0000)"] = comparator
	minDelegationFee, err := app.Prompt.CaptureUint64Compare("Minimum Delegation Fee", comparatorMap)
	if err != nil {
		return 0, err
	}
	return uint32(minDelegationFee), nil
}

func getMinDelegatorStake() (uint64, error) {
	ux.Logger.PrintToUser("Select the Minimum Delegator Stake")
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = 0
	comparatorMap["0"] = comparator
	minDelegatorStake, err := app.Prompt.CaptureUint64Compare("Minimum Delegator Stake", comparatorMap)
	if err != nil {
		return 0, err
	}
	return minDelegatorStake, nil
}

func getMaxValidatorWeightFactor(tokenName string, initialSupply uint64) (uint64, error) {
	ux.Logger.PrintToUser("Select the Maximum Validator Weight")
	maxSupply, err := app.Prompt.CaptureUint64Compare("Maximum Supply amount", initialSupply, "Initial Supply")
	if err != nil {
		return 0, err
	}
	return maxSupply, nil
}

//func getUptimeRequirement(tokenName string, initialSupply uint64) (uint64, error) {
//	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Supply of %s", tokenName))
//	maxSupply, err := app.Prompt.CaptureUint64Compare("Maximum Supply amount", initialSupply, "Initial Supply")
//	if err != nil {
//		return 0, err
//	}
//	return maxSupply, nil
//}

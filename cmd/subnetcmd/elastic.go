// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

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
	defaultMinValidatorStake        = 2_000
	defaultMaxValidatorStake        = 3_000_000
	defaultMinStakeDurationHours    = 14 * 24
	defaultMinStakeDuration         = defaultMinStakeDurationHours * time.Hour
	defaultMaxStakeDurationHours    = 365 * 24
	defaultMaxStakeDuration         = defaultMaxStakeDurationHours * time.Hour
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

func getElasticSubnetConfig(tokenSymbol string) (models.ElasticSubnetConfig, error) {
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
	}
	customElasticSubnetConfig, err := getCustomElasticSubnetConfig(tokenSymbol)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	return customElasticSubnetConfig, nil
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
	elasticSubnetConfig, err := getElasticSubnetConfig(tokenSymbol)
	if err != nil {
		return err
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
			deployer := subnet.NewPublicDeployer(app, false, keyChain, models.Local)
			txID, assetID, err := deployer.IssueTransformSubnetTx(elasticSubnetConfig, subnetID, tokenName, tokenSymbol)
			if err != nil {
				return err
			}
			elasticSubnetConfig.AssetID = assetID
			PrintTransformResults(subnetName, txID, subnetID, tokenName, tokenSymbol, assetID)
			if err = app.CreateElasticSubnetConfig(subnetName, &elasticSubnetConfig); err != nil {
				cancel <- struct{}{}
				return err
			}
		}
	}

	return nil
}

func getCustomElasticSubnetConfig(tokenSymbol string) (models.ElasticSubnetConfig, error) {
	ux.Logger.PrintToUser("More info regarding elastic subnet parameters can be found at https://docs.avax.network/subnets/reference-elastic-subnets-parameters")
	initialSupply, err := getInitialSupply(tokenSymbol)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	maxSupply, err := getMaximumSupply(tokenSymbol, initialSupply)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minConsumptionRate, maxConsumptionRate, err := getConsumptionRate()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minValidatorStake, maxValidatorStake, err := getValidatorStake(initialSupply, maxSupply)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minStakeDuration, maxStakeDuration, err := getStakeDuration()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minDelegationFee, err := getMinDelegationFee()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minDelegatorStake, err := getMinDelegatorStake()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	maxValidatorWeightFactor, err := getMaxValidatorWeightFactor()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	uptimeReq, err := getUptimeRequirement()
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}

	elasticSubnetConfig := models.ElasticSubnetConfig{
		InitialSupply:            initialSupply,
		MaxSupply:                maxSupply,
		MinConsumptionRate:       minConsumptionRate,
		MaxConsumptionRate:       maxConsumptionRate,
		MinValidatorStake:        minValidatorStake,
		MaxValidatorStake:        maxValidatorStake,
		MinStakeDuration:         minStakeDuration,
		MaxStakeDuration:         maxStakeDuration,
		MinDelegationFee:         minDelegationFee,
		MinDelegatorStake:        minDelegatorStake,
		MaxValidatorWeightFactor: maxValidatorWeightFactor,
		UptimeRequirement:        uptimeReq,
	}
	return elasticSubnetConfig, err
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

func getInitialSupply(tokenName string) (uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Initial Supply of %s. \"_\" can be used as thousand separator", tokenName))
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Initial Supply is %s", ux.ConvertToStringWithThousandSeparator(defaultInitialSupply)))
	initialSupply, err := app.Prompt.CaptureUint64("Initial Supply amount")
	if err != nil {
		return 0, err
	}
	return initialSupply, nil
}

func getMaximumSupply(tokenName string, initialSupply uint64) (uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Maximum Supply of %s. \"_\" can be used as thousand separator", tokenName))
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Supply is %s", ux.ConvertToStringWithThousandSeparator(defaultMaximumSupply)))
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
	ux.Logger.PrintToUser("Select the Minimum Consumption Rate. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Consumption Rate is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMinConsumptionRate*reward.PercentDenominator))))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = reward.PercentDenominator
	comparatorMap["Percent Denominator(1_0000_0000)"] = comparator
	minConsumptionRate, err := app.Prompt.CaptureUint64Compare("Minimum Consumption Rate", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser("Select the Maximum Consumption Rate. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Consumption Rate is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMaxConsumptionRate*reward.PercentDenominator))))
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
	ux.Logger.PrintToUser("Select the Minimum Validator Stake. \"_\" can be used as thousand separator")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Validator Stake is %s", ux.ConvertToStringWithThousandSeparator(defaultMinValidatorStake)))
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

	ux.Logger.PrintToUser("Select the Maximum Validator Stake. \"_\" can be used as thousand separator")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Validator Stake is %s", ux.ConvertToStringWithThousandSeparator(defaultMaxValidatorStake)))
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

func getStakeDuration() (time.Duration, time.Duration, error) {
	ux.Logger.PrintToUser("Select the Minimum Stake Duration. Please enter in units of hours")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Stake Duration is %d (14 x 24)", defaultMinStakeDurationHours))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = 0
	comparatorMap["0"] = comparator
	minStakeDuration, err := app.Prompt.CaptureUint64Compare("Minimum Stake Duration", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser("Select the Maximum Stake Duration")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Stake Duration is %d (365 x 24)", defaultMaxStakeDurationHours))
	comparatorMap = map[string]prompts.Comparator{}
	comparator = prompts.Comparator{}
	comparator.CompareType = prompts.MoreThanEq
	comparator.CompareValue = minStakeDuration
	comparatorMap["Minimum Stake Duration"] = comparator
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = uint64(defaultMaxStakeDurationHours)
	comparatorMap["Global Max Stake Duration"] = comparator
	maxStakeDuration, err := app.Prompt.CaptureUint64Compare("Maximum Stake Duration", comparatorMap)
	if err != nil {
		return 0, 0, err
	}

	return time.Duration(minStakeDuration) * time.Hour, time.Duration(maxStakeDuration) * time.Hour, nil
}

func getMinDelegationFee() (uint32, error) {
	ux.Logger.PrintToUser("Select the Minimum Delegation Fee. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Delegation Fee is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMinDelegationFee))))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = reward.PercentDenominator
	comparatorMap["Percent Denominator(1_0000_0000)"] = comparator
	minDelegationFee, err := app.Prompt.CaptureUint64Compare("Minimum Delegation Fee", comparatorMap)
	if err != nil {
		return 0, err
	}
	if minDelegationFee > math.MaxInt32 {
		return 0, fmt.Errorf("minimum Delegation Fee needs to be unsigned 32-bit integer")
	}
	return uint32(minDelegationFee), nil
}

func getMinDelegatorStake() (uint64, error) {
	ux.Logger.PrintToUser("Select the Minimum Delegator Stake")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Delegator Stake is %d", defaultMinDelegatorStake))
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

func getMaxValidatorWeightFactor() (byte, error) {
	ux.Logger.PrintToUser("Select the Maximum Validator Weight Factor. A value of 1 effectively disables delegation")
	ux.Logger.PrintToUser("More info can be found at https://docs.avax.network/subnets/reference-elastic-subnets-parameters#delegators-weight-checks")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Validator Weight Factor is %d", defaultMaxValidatorWeightFactor))
	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.MoreThan
	comparator.CompareValue = 0
	comparatorMap["0"] = comparator
	maxValidatorWeightFactor, err := app.Prompt.CaptureUint64Compare("Maximum Validator Weight Factor", comparatorMap)
	if err != nil {
		return 0, err
	}
	if maxValidatorWeightFactor > math.MaxInt8 {
		return 0, fmt.Errorf("maximum Validator Weight Factor needs to be unsigned 8-bit integer")
	}
	return byte(maxValidatorWeightFactor), nil
}

func getUptimeRequirement() (uint32, error) {
	ux.Logger.PrintToUser("Select the Uptime Requirement. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Uptime Requirement is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultUptimeRequirement*reward.PercentDenominator))))

	comparatorMap := map[string]prompts.Comparator{}
	comparator := prompts.Comparator{}
	comparator.CompareType = prompts.LessThanEq
	comparator.CompareValue = reward.PercentDenominator
	comparatorMap["Percent Denominator(1_0000_0000)"] = comparator
	uptimeReq, err := app.Prompt.CaptureUint64Compare("Uptime Requirement", comparatorMap)
	if err != nil {
		return 0, err
	}
	if uptimeReq > math.MaxInt32 {
		return 0, fmt.Errorf("uptime Requirement needs to be unsigned 32-bit integer")
	}
	return uint32(uptimeReq), nil
}

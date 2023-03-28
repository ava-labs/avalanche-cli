// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package elasticsubnet

import (
	"fmt"
	"math"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
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
)

func GetElasticSubnetConfig(app *application.Avalanche, tokenSymbol string) (models.ElasticSubnetConfig, error) {
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
	customElasticSubnetConfig, err := getCustomElasticSubnetConfig(app, tokenSymbol)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	return customElasticSubnetConfig, nil
}

func getCustomElasticSubnetConfig(app *application.Avalanche, tokenSymbol string) (models.ElasticSubnetConfig, error) {
	ux.Logger.PrintToUser("More info regarding elastic subnet parameters can be found at https://docs.avax.network/subnets/reference-elastic-subnets-parameters")
	initialSupply, err := getInitialSupply(app, tokenSymbol)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	maxSupply, err := getMaximumSupply(app, tokenSymbol, initialSupply)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minConsumptionRate, maxConsumptionRate, err := getConsumptionRate(app)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minValidatorStake, maxValidatorStake, err := getValidatorStake(app, initialSupply, maxSupply)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minStakeDuration, maxStakeDuration, err := getStakeDuration(app)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minDelegationFee, err := getMinDelegationFee(app)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	minDelegatorStake, err := getMinDelegatorStake(app)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	maxValidatorWeightFactor, err := getMaxValidatorWeightFactor(app)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}
	uptimeReq, err := getUptimeRequirement(app)
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

func getInitialSupply(app *application.Avalanche, tokenName string) (uint64, error) {
	ux.Logger.PrintToUser(fmt.Sprintf("Select the Initial Supply of %s. \"_\" can be used as thousand separator", tokenName))
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Initial Supply is %s", ux.ConvertToStringWithThousandSeparator(defaultInitialSupply)))
	initialSupply, err := app.Prompt.CaptureUint64("Initial Supply amount")
	if err != nil {
		return 0, err
	}
	return initialSupply, nil
}

func getMaximumSupply(app *application.Avalanche, tokenName string, initialSupply uint64) (uint64, error) {
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

func getConsumptionRate(app *application.Avalanche) (uint64, uint64, error) {
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

func getValidatorStake(app *application.Avalanche, initialSupply uint64, maximumSupply uint64) (uint64, uint64, error) {
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

func getStakeDuration(app *application.Avalanche) (time.Duration, time.Duration, error) {
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

func getMinDelegationFee(app *application.Avalanche) (uint32, error) {
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

func getMinDelegatorStake(app *application.Avalanche) (uint64, error) {
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

func getMaxValidatorWeightFactor(app *application.Avalanche) (byte, error) {
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

func getUptimeRequirement(app *application.Avalanche) (uint32, error) {
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

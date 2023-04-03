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

// default elastic config parameter values are from
// https://docs.avax.network/subnets/reference-elastic-subnets-parameters#primary-network-parameters-on-mainnet
const (
	defaultInitialSupply               = 240_000_000
	defaultMaximumSupply               = 720_000_000
	defaultMinConsumptionRate          = 0.1
	defaultMaxConsumptionRate          = 0.12
	defaultMinValidatorStake           = 2_000
	defaultMaxValidatorStake           = 3_000_000
	defaultMinStakeDurationHours       = 14 * 24
	defaultMinStakeDurationHoursString = "14 x 24"
	defaultMinStakeDuration            = defaultMinStakeDurationHours * time.Hour
	defaultMaxStakeDurationHours       = 365 * 24
	defaultMaxStakeDurationHoursString = "365 x 24"
	defaultMaxStakeDuration            = defaultMaxStakeDurationHours * time.Hour
	defaultMinDelegationFee            = 20_000
	defaultMinDelegatorStake           = 25
	defaultMaxValidatorWeightFactor    = 5
	defaultUptimeRequirement           = 0.8
)

func GetElasticSubnetConfig(app *application.Avalanche, tokenSymbol string, useDefaultConfig bool) (models.ElasticSubnetConfig, error) {
	const (
		defaultConfig   = "Use default elastic subnet config"
		customizeConfig = "Customize elastic subnet config"
	)
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
	if useDefaultConfig {
		return elasticSubnetConfig, nil
	}
	elasticSubnetConfigOptions := []string{defaultConfig, customizeConfig}
	chosenConfig, err := app.Prompt.CaptureList(
		"How would you like to set fees",
		elasticSubnetConfigOptions,
	)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
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
	maxSupply, err := app.Prompt.CaptureUint64Compare(
		"Maximum Supply amount",
		map[string]prompts.Comparator{
			"Initial Supply": {
				CompareType:  prompts.MoreThanEq,
				CompareValue: initialSupply,
			},
		})
	if err != nil {
		return 0, err
	}
	return maxSupply, nil
}

func getConsumptionRate(app *application.Avalanche) (uint64, uint64, error) {
	ux.Logger.PrintToUser("Select the Minimum Consumption Rate. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Consumption Rate is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMinConsumptionRate*reward.PercentDenominator))))
	minConsumptionRate, err := app.Prompt.CaptureUint64Compare(
		"Minimum Consumption Rate",
		map[string]prompts.Comparator{
			"Percent Denominator(1_0000_0000)": {
				CompareType:  prompts.LessThanEq,
				CompareValue: reward.PercentDenominator,
			},
		})
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser("Select the Maximum Consumption Rate. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Consumption Rate is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMaxConsumptionRate*reward.PercentDenominator))))
	maxConsumptionRate, err := app.Prompt.CaptureUint64Compare(
		"Maximum Consumption Rate",
		map[string]prompts.Comparator{
			"Percent Denominator(1_0000_0000)": {
				CompareType:  prompts.LessThanEq,
				CompareValue: reward.PercentDenominator,
			},
			"Mininum Consumption Rate": {
				CompareType:  prompts.MoreThanEq,
				CompareValue: minConsumptionRate,
			},
		})
	if err != nil {
		return 0, 0, err
	}
	return minConsumptionRate, maxConsumptionRate, nil
}

func getValidatorStake(app *application.Avalanche, initialSupply uint64, maximumSupply uint64) (uint64, uint64, error) {
	ux.Logger.PrintToUser("Select the Minimum Validator Stake. \"_\" can be used as thousand separator")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Validator Stake is %s", ux.ConvertToStringWithThousandSeparator(defaultMinValidatorStake)))
	minValidatorStake, err := app.Prompt.CaptureUint64Compare(
		"Minimum Validator Stake",
		map[string]prompts.Comparator{
			"Initial Supply": {
				CompareType:  prompts.LessThanEq,
				CompareValue: initialSupply,
			},
			"0": {
				CompareType:  prompts.MoreThan,
				CompareValue: 0,
			},
		})
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser("Select the Maximum Validator Stake. \"_\" can be used as thousand separator")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Validator Stake is %s", ux.ConvertToStringWithThousandSeparator(defaultMaxValidatorStake)))
	maxValidatorStake, err := app.Prompt.CaptureUint64Compare(
		"Maximum Validator Stake",
		map[string]prompts.Comparator{
			"Maximum Supply": {
				CompareType:  prompts.LessThanEq,
				CompareValue: maximumSupply,
			},
			"Minimum Validator Stake": {
				CompareType:  prompts.MoreThan,
				CompareValue: minValidatorStake,
			},
		})
	if err != nil {
		return 0, 0, err
	}
	return minValidatorStake, maxValidatorStake, nil
}

func getStakeDuration(app *application.Avalanche) (time.Duration, time.Duration, error) {
	ux.Logger.PrintToUser("Select the Minimum Stake Duration. Please enter in units of hours")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Stake Duration is %d (%s)", defaultMinStakeDurationHours, defaultMinStakeDurationHoursString))
	minStakeDuration, err := app.Prompt.CaptureUint64Compare(
		"Minimum Stake Duration",
		map[string]prompts.Comparator{
			"0": {
				CompareType:  prompts.MoreThan,
				CompareValue: 0,
			},
			"Global Max Stake Duration": {
				CompareType:  prompts.LessThanEq,
				CompareValue: uint64(defaultMaxStakeDurationHours),
			},
		})
	if err != nil {
		return 0, 0, err
	}

	ux.Logger.PrintToUser("Select the Maximum Stake Duration")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Stake Duration is %d (%s)", defaultMaxStakeDurationHours, defaultMaxStakeDurationHoursString))
	maxStakeDuration, err := app.Prompt.CaptureUint64Compare(
		"Maximum Stake Duration",
		map[string]prompts.Comparator{
			"Minimum Stake Duration": {
				CompareType:  prompts.MoreThanEq,
				CompareValue: minStakeDuration,
			},
			"Global Max Stake Duration": {
				CompareType:  prompts.LessThanEq,
				CompareValue: uint64(defaultMaxStakeDurationHours),
			},
		})
	if err != nil {
		return 0, 0, err
	}

	return time.Duration(minStakeDuration) * time.Hour, time.Duration(maxStakeDuration) * time.Hour, nil
}

func getMinDelegationFee(app *application.Avalanche) (uint32, error) {
	ux.Logger.PrintToUser("Select the Minimum Delegation Fee. Please denominate your percentage in PercentDenominator")
	ux.Logger.PrintToUser("To denominate your percentage in PercentDenominator just multiply it by 10_000. For example, 1 percent corresponds to 10_000")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Minimum Delegation Fee is %s", ux.ConvertToStringWithThousandSeparator(uint64(defaultMinDelegationFee))))
	minDelegationFee, err := app.Prompt.CaptureUint64Compare(
		"Minimum Delegation Fee",
		map[string]prompts.Comparator{
			"Percent Denominator(1_0000_0000)": {
				CompareType:  prompts.LessThanEq,
				CompareValue: reward.PercentDenominator,
			},
		})
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
	minDelegatorStake, err := app.Prompt.CaptureUint64Compare(
		"Minimum Delegator Stake",
		map[string]prompts.Comparator{
			"0": {
				CompareType:  prompts.MoreThan,
				CompareValue: 0,
			},
		})
	if err != nil {
		return 0, err
	}
	return minDelegatorStake, nil
}

func getMaxValidatorWeightFactor(app *application.Avalanche) (byte, error) {
	ux.Logger.PrintToUser("Select the Maximum Validator Weight Factor. A value of 1 effectively disables delegation")
	ux.Logger.PrintToUser("More info can be found at https://docs.avax.network/subnets/reference-elastic-subnets-parameters#delegators-weight-checks")
	ux.Logger.PrintToUser(fmt.Sprintf("Mainnet Maximum Validator Weight Factor is %d", defaultMaxValidatorWeightFactor))
	maxValidatorWeightFactor, err := app.Prompt.CaptureUint64Compare(
		"Maximum Validator Weight Factor",
		map[string]prompts.Comparator{
			"0": {
				CompareType:  prompts.MoreThan,
				CompareValue: 0,
			},
		})
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
	uptimeReq, err := app.Prompt.CaptureUint64Compare(
		"Uptime Requirement",
		map[string]prompts.Comparator{
			"Percent Denominator(1_0000_0000)": {
				CompareType:  prompts.LessThanEq,
				CompareValue: reward.PercentDenominator,
			},
		})
	if err != nil {
		return 0, err
	}
	if uptimeReq > math.MaxInt32 {
		return 0, fmt.Errorf("uptime Requirement needs to be unsigned 32-bit integer")
	}
	return uint32(uptimeReq), nil
}

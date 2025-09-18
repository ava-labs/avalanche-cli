// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validator"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/libevm/common"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [blockchainName]",
		Short: "Lists the validators of an L1",
		Long:  `This command gets a list of the validators of the L1`,
		RunE:  list,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	return cmd
}

func list(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	if !sc.Sovereign {
		return fmt.Errorf("avalanche validator commands are only applicable to sovereign L1s")
	}

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

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return err
	}

	validators, err := validator.GetCurrentValidators(network.SDKNetwork(), subnetID)
	if err != nil {
		return err
	}

	validatorManagerRPCEndpoint := sc.Networks[network.Name()].ValidatorManagerRPCEndpoint
	validatorManagerAddress := sc.Networks[network.Name()].ValidatorManagerAddress
	specializedValidatorManagerAddress := sc.Networks[network.Name()].SpecializedValidatorManagerAddress
	if specializedValidatorManagerAddress != "" {
		validatorManagerAddress = specializedValidatorManagerAddress
	}

	header := table.Row{"Node ID", "Validation ID", "Weight", "Remaining Balance (AVAX)", "Owner", "StartTime"}
	if sc.PoS() {
		header = table.Row{"Node ID", "Validation ID", "Weight", "Remaining Balance (AVAX)", "Owner", "StartTime", "Stake", "EndTime", "Ready"}
	}
	t := ux.DefaultTable(
		fmt.Sprintf("%s Validators", blockchainName),
		header,
	)
	for _, validator := range validators {
		validatorInfo, err := validatormanager.GetValidator(
			validatorManagerRPCEndpoint,
			common.HexToAddress(validatorManagerAddress),
			validator.ValidationID,
		)
		if err != nil {
			return err
		}
		startTime := time.Unix(int64(validatorInfo.StartTime), 0)
		startTimeStr := startTime.Format("2006-01-02 15:04:05")
		endTimeStr := ""
		readyToRemove := ""
		owner := sc.ValidatorManagerOwner
		stake := "0"
		if sc.PoS() {
			stakingValidatorInfo, err := validatormanager.GetStakingValidator(
				validatorManagerRPCEndpoint,
				common.HexToAddress(validatorManagerAddress),
				validator.ValidationID,
			)
			if err != nil {
				return err
			}
			if stakingValidatorInfo.MinStakeDuration != 0 {
				endTime := startTime.Add(time.Second * time.Duration(stakingValidatorInfo.MinStakeDuration))
				endTimeStr = endTime.Format("2006-01-02 15:04:05")
				readyToRemove = fmt.Sprintf("%v", time.Now().After(endTime))
				owner = stakingValidatorInfo.Owner.Hex()
				stakeAmount, err := validatormanager.PoSWeightToValue(
					validatorManagerRPCEndpoint,
					common.HexToAddress(validatorManagerAddress),
					uint64(validator.Weight),
				)
				if err != nil {
					return fmt.Errorf("failure obtaining value from weight: %w", err)
				}
				stake = utils.FormatAmount(stakeAmount, 18)
			}
		}
		row := table.Row{
			validator.NodeID,
			validator.ValidationID,
			validator.Weight,
			float64(validator.Balance) / float64(units.Avax),
			owner,
			startTimeStr,
		}
		if sc.PoS() {
			row = table.Row{
				validator.NodeID,
				validator.ValidationID,
				validator.Weight,
				float64(validator.Balance) / float64(units.Avax),
				owner,
				startTimeStr,
				stake,
				endTimeStr,
				readyToRemove,
			}
		}
		t.AppendRow(row)
	}
	fmt.Println(t.Render())

	return nil
}

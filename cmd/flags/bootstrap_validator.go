// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	bootstrapFilepathFlag      = "bootstrap-filepath"
	generateNodeIDFlag         = "generate-node-id"
	bootstrapEndpointsFlag     = "bootstrap-endpoints"
	numBootstrapValidatorsFlag = "num-bootstrap-validators"
	balanceFlag                = "balance"
	changeOwnerAddressFlag     = "change-owner-address"
)

type BootstrapValidatorFlags struct {
	BootstrapValidatorsJSONFilePath string
	GenerateNodeID                  bool
	BootstrapEndpoints              []string
	NumBootstrapValidators          int
	DeployBalanceAVAX               float64
	ChangeOwnerAddress              string
}

func validateBootstrapFilepathFlag(cmd *cobra.Command, bootstrapValidatorFlags BootstrapValidatorFlags) error {
	if bootstrapValidatorFlags.BootstrapValidatorsJSONFilePath != "" {
		if bootstrapValidatorFlags.GenerateNodeID {
			return fmt.Errorf("cannot use --generate-node-id=true and a non-empty --bootstrap-filepath at the same time")
		}
		if cmd.Flags().Changed(numBootstrapValidatorsFlag) {
			return fmt.Errorf("cannot use a non-empty --num-bootstrap-validators and a non-empty --bootstrap-filepath at the same time")
		}
		if cmd.Flags().Changed(balanceFlag) {
			return fmt.Errorf("cannot use a non-empty --balance and a non-empty --bootstrap-filepath at the same time")
		}
		if bootstrapValidatorFlags.BootstrapEndpoints != nil {
			return fmt.Errorf("cannot use a non-empty --bootstrap-endpoints and a non-empty --bootstrap-filepath at the same time")
		}
	}
	return nil
}

func validateBootstrapEndpointFlag(cmd *cobra.Command, bootstrapValidatorFlags BootstrapValidatorFlags) error {
	if bootstrapValidatorFlags.BootstrapEndpoints != nil {
		if bootstrapValidatorFlags.GenerateNodeID {
			return fmt.Errorf("cannot use --generate-node-id=true and a non-empty --bootstrap-endpoints at the same time")
		}
		if cmd.Flags().Changed(numBootstrapValidatorsFlag) {
			return fmt.Errorf("cannot use a non-empty --num-bootstrap-validators and a non-empty --bootstrap-endpoints at the same time")
		}
	}
	return nil
}

func validateBootstrapValidatorFlags(cmd *cobra.Command, bootstrapValidatorFlags BootstrapValidatorFlags) error {
	if err := validateBootstrapFilepathFlag(cmd, bootstrapValidatorFlags); err != nil {
		return err
	}
	return validateBootstrapEndpointFlag(cmd, bootstrapValidatorFlags)
}

func AddBootstrapValidatorFlagsToCmd(cmd *cobra.Command, bootstrapFlags *BootstrapValidatorFlags) GroupedFlags {
	return RegisterFlagGroup(cmd, "Bootstrap Validators Flags", "show-bootstrap-validators-flags", true, func(set *pflag.FlagSet) {
		set.StringVar(&bootstrapFlags.BootstrapValidatorsJSONFilePath, bootstrapFilepathFlag, "", "JSON file path that provides details about bootstrap validators")
		set.BoolVar(&bootstrapFlags.GenerateNodeID, generateNodeIDFlag, false, "set to true to generate Node IDs for bootstrap validators when none are set up. Use these Node IDs to set up your Avalanche Nodes.")
		set.StringSliceVar(&bootstrapFlags.BootstrapEndpoints, bootstrapEndpointsFlag, nil, "take validator node info from the given endpoints")
		set.IntVar(&bootstrapFlags.NumBootstrapValidators, numBootstrapValidatorsFlag, 0, "number of bootstrap validators to set up in sovereign L1 validator")
		set.Float64Var(
			&bootstrapFlags.DeployBalanceAVAX,
			balanceFlag,
			float64(constants.BootstrapValidatorBalanceNanoAVAX)/float64(units.Avax),
			"set the AVAX balance of each bootstrap validator that will be used for continuous fee on P-Chain (setting balance=1 equals to 1 AVAX for each bootstrap validator)",
		)
		set.StringVar(&bootstrapFlags.ChangeOwnerAddress, changeOwnerAddressFlag, "", "address that will receive change if node is no longer L1 validator")
		bootstrapValidatorPreRun := func(cmd *cobra.Command, _ []string) error {
			if err := validateBootstrapValidatorFlags(cmd, *bootstrapFlags); err != nil {
				return err
			}
			//if bootstrapValidatorsJSONFilePath != "" {
			//	bootstrapValidators, err = LoadBootstrapValidator(bootstrapValidatorsJSONFilePath)
			//	if err != nil {
			//		return err
			//	}
			//	numBootstrapValidators = len(bootstrapValidators)
			//}

			// TODO: incorporate this
			if bootstrapFlags.DeployBalanceAVAX <= 0 {
				return fmt.Errorf("bootstrap validator balance must be greater than 0 AVAX")
			}
			//TODO: incorporate this
			//if bootstrapValidatorFlags.ChangeOwnerAddress == "" {
			//	// use provided key as change owner unless already set
			//	if pAddr, err := kc.PChainFormattedStrAddresses(); err == nil && len(pAddr) > 0 {
			//		bootstrapValidatorFlags.ChangeOwnerAddress = pAddr[0]
			//		ux.Logger.PrintToUser("Using [%s] to be set as a change owner for leftover AVAX", *changeOwnerAddress)
			//	}
			//}
			return nil
		}

		existingPreRunE := cmd.PreRunE
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			if existingPreRunE != nil {
				if err := existingPreRunE(cmd, args); err != nil {
					return err
				}
			}
			return bootstrapValidatorPreRun(cmd, args)
		}
	})
}

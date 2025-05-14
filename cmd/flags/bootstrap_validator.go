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

func validateBootstrapValidatorFlags(localMachineFlags LocalMachineFlags) error {
	if len(localMachineFlags.StakingSignerKeyPaths) != len(localMachineFlags.StakingCertKeyPaths) || len(localMachineFlags.StakingSignerKeyPaths) != len(localMachineFlags.StakingTLSKeyPaths) {
		return fmt.Errorf("staking key inputs must be for the same number of nodes")
	}
	return nil
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
	})
}

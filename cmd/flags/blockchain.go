// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

const (
	rpcURLFLag         = "rpc"
	authKeysFlag       = "auth-keys"
	outputTxPathFlag   = "output-tx-path"
	sameControlKeyFlag = "same-control-key"
	controlKeysFlag    = "control-keys"
	thresholdFlag      = "threshold"
)

var RPC string

func AddRPCFlagToCmd(cmd *cobra.Command) {
	cmd.Flags().StringVar(&RPC, rpcURLFLag, "", "blockchain rpc endpoint")
}

var NonSovFlags SubnetFlags

type SubnetFlags struct {
	SubnetAuthKeys []string
	OutputTxPath   string
	SameControlKey bool
	ControlKeys    []string
	Threshold      uint32
}

//func AddNonSovFlagsToCmd(cmd *cobra.Command) {
//	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
//	cmd.Flags().DurationVar(&duration, "staking-period", 0, "how long this validator will be staking")
//	cmd.Flags().BoolVar(&useDefaultStartTime, "default-start-time", false, "(for Subnets, not L1s) use default start time for subnet validator (5 minutes later for fuji & mainnet, 30 seconds later for devnet)")
//	cmd.Flags().StringVar(&startTimeStr, "start-time", "", "(for Subnets, not L1s) UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format")
//	cmd.Flags().BoolVar(&useDefaultDuration, "default-duration", false, "(for Subnets, not L1s) set duration so as to validate until primary validator ends its period")
//	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "(for Subnets, not L1s) use default weight/start/duration params for subnet validator")
//	cmd.Flags().StringSliceVar(&subnetAuthKeys, "subnet-auth-keys", nil, "(for Subnets, not L1s) control keys that will be used to authenticate add validator tx")
//	cmd.Flags().StringVar(&outputTxPath, "output-tx-path", "", "(for Subnets, not L1s) file path of the add validator tx")
//	cmd.Flags().BoolVar(&waitForTxAcceptance, "wait-for-tx-acceptance", true, "(for Subnets, not L1s) just issue the add validator tx, without waiting for its acceptance")
//}

func AddNonSovFlagsToCmd(cmd *cobra.Command, includeControlKeys bool) {
	cmd.Flags().StringSliceVar(&NonSovFlags.SubnetAuthKeys, authKeysFlag, nil, "(for non-SOV blockchain only) control keys that will be used to authenticate the removeValidator tx")
	cmd.Flags().StringVar(&NonSovFlags.OutputTxPath, outputTxPathFlag, "", "(for non-SOV blockchain only) file path of the removeValidator tx")
	if includeControlKeys {
		cmd.Flags().BoolVarP(&NonSovFlags.SameControlKey, sameControlKeyFlag, "s", false, "use the fee-paying key as control key")
		cmd.Flags().StringSliceVar(&NonSovFlags.ControlKeys, controlKeysFlag, nil, "addresses that may make blockchain changes")
		cmd.Flags().Uint32Var(&NonSovFlags.Threshold, thresholdFlag, 0, "required number of control key signatures to make blockchain changes")
	}
}

func (nonSovFlags *SubnetFlags) ValidateOutputTxPath() error {
	if nonSovFlags.OutputTxPath != "" {
		if _, err := os.Stat(nonSovFlags.OutputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", nonSovFlags.OutputTxPath)
		}
	}
	return nil
}

// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	useLocalMachineFlag      = "use-local-machine"
	partialSyncFlag          = "partial-sync"
	httpPortFlag             = "http-port"
	stakingPortFlag          = "staking-port"
	avalanchegoPathFlag      = "avalanchego-path"
	avalanchegoVersionFlag   = "avalanchego-version"
	stakingTLSKeyPathFlag    = "staking-tls-key-path"
	stakingCertKeyPathFlag   = "staking-cert-key-path"
	stakingSignerKeyPathFlag = "staking-signer-key-path"
)

type LocalMachineFlags struct {
	UseLocalMachine          bool
	PartialSync              bool
	HTTPPorts                []uint
	StakingPorts             []uint
	AvagoBinaryPath          string
	UserProvidedAvagoVersion string
	StakingTLSKeyPaths       []string
	StakingCertKeyPaths      []string
	StakingSignerKeyPaths    []string
}

func validateLocalMachineFlags(localMachineFlags LocalMachineFlags) error {
	if len(localMachineFlags.StakingSignerKeyPaths) != len(localMachineFlags.StakingCertKeyPaths) || len(localMachineFlags.StakingSignerKeyPaths) != len(localMachineFlags.StakingTLSKeyPaths) {
		return fmt.Errorf("staking key inputs must be for the same number of nodes")
	}
	return nil
}

func AddLocalMachineFlagsToCmd(cmd *cobra.Command, localMachineFlags *LocalMachineFlags) GroupedFlags {
	return RegisterFlagGroup(cmd, "Local Machine Flags (Use Local Machine as Bootstrap Validator)", "show-local-machine-flags", false, func(set *pflag.FlagSet) {
		set.BoolVar(&localMachineFlags.UseLocalMachine, useLocalMachineFlag, false, "use local machine as a blockchain validator")
		set.BoolVar(&localMachineFlags.PartialSync, partialSyncFlag, true, "set primary network partial sync for new validators")
		set.UintSliceVar(&localMachineFlags.HTTPPorts, httpPortFlag, []uint{}, "http port for node(s)")
		set.UintSliceVar(&localMachineFlags.StakingPorts, stakingPortFlag, []uint{}, "staking port for node(s)")
		set.StringVar(&localMachineFlags.AvagoBinaryPath, avalanchegoPathFlag, "", "use this avalanchego binary path")
		set.StringVar(
			&localMachineFlags.UserProvidedAvagoVersion,
			avalanchegoVersionFlag,
			constants.DefaultAvalancheGoVersion,
			"use this version of avalanchego (ex: v1.17.12)",
		)
		set.StringSliceVar(&localMachineFlags.StakingTLSKeyPaths, stakingTLSKeyPathFlag, []string{}, "path to provided staking TLS key for node(s)")
		set.StringSliceVar(&localMachineFlags.StakingCertKeyPaths, stakingCertKeyPathFlag, []string{}, "path to provided staking cert key for node(s)")
		set.StringSliceVar(&localMachineFlags.StakingSignerKeyPaths, stakingSignerKeyPathFlag, []string{}, "path to provided staking signer key for node(s)")
		localMachinePreRun := func(_ *cobra.Command, _ []string) error {
			if err := validateLocalMachineFlags(*localMachineFlags); err != nil {
				return err
			}
			return nil
		}

		existingPreRunE := cmd.PreRunE
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			if existingPreRunE != nil {
				if err := existingPreRunE(cmd, args); err != nil {
					return err
				}
			}
			return localMachinePreRun(cmd, args)
		}
	})
}

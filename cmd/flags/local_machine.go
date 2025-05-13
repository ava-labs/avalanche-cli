// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

const (
	UseLocalMachine      = "use-local-machine"
	partialSync          = "partial-sync"
	httpPort             = "http-port"
	stakingPort          = "staking-port"
	avalanchegoPath      = "avalanchego-path"
	avalanchegoVersion   = "avalanchego-version"
	stakingTLSKeyPath    = "staking-tls-key-path"
	stakingCertKeyPath   = "staking-cert-key-path"
	stakingSignerKeyPath = "staking-signer-key-path"
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

func AddLocalMachineFlagsToCmd(cmd *cobra.Command, localMachineFlags *LocalMachineFlags) {
	set.BoolVar(&LocalMachineFlags.Uselo, "use-local-machine", false, "use local machine as a blockchain validator")

	cmd.Flags().StringVar(&sigAggFlags.AggregatorLogLevel, aggregatorLogLevelFlag, constants.DefaultAggregatorLogLevel, "log level to use with signature aggregator")
	cmd.Flags().BoolVar(&sigAggFlags.AggregatorLogToStdout, aggregatorLogToStdoutFlag, false, "use stdout for signature aggregator logs")
}

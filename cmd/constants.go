// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

// CommandGroup represents the different command groups available in the CLI
type CommandGroup string

const (
	BlockchainCmd CommandGroup = "blockchain"
	ValidatorCmd  CommandGroup = "validator"
	ICMCmd        CommandGroup = "icm"
	InterchainCmd CommandGroup = "interchain"
)

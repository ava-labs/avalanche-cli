// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

var (
	migrations = map[int]func() error{0: migrateTopLevelFiles}
	showMsg    = true
	migRunning = false
)

// poor-man's migrations: there are no rollbacks (for now)
func runMigrations() error {
	// by using an int index we can sort of "enforce" an order
	// with just an array it could easily happen that someone
	// prepends a new migration at the front instead of the bottom
	for i := 0; i < len(migrations); i++ {
		err := migrations[i]()
		if err != nil {
			return fmt.Errorf("migration #%d failed: %w", i, err)
		}
	}
	if migRunning {
		ux.Logger.PrintToUser("Update process successfully completed")
		migRunning = false
	}
	return nil
}

// Up to version 1.0.0 the sidecar and genesis files were stored at
// {baseDir} in the top-level.
// Due to new requirements and evolution of the tool, we now store
// every subnet-specific file in {baseDir}/subnets/{subnetName}
func migrateTopLevelFiles() error {
	baseDir := app.GetBaseDir()
	sidecarMatches, err := filepath.Glob(filepath.Join(baseDir, "*"+constants.SidecarSuffix))
	if err != nil {
		return err
	}
	genesisMatches, err := filepath.Glob(filepath.Join(baseDir, "*"+constants.GenesisSuffix))
	if err != nil {
		return err
	}
	if len(sidecarMatches) > 0 || len(genesisMatches) > 0 {
		printMigrationMessage()
	}

	//nolint: gocritic
	allMatches := append(sidecarMatches, genesisMatches...)
	var subnet, suffix, fileName string
	for _, m := range allMatches {
		fileName = filepath.Base(m)
		parts := strings.Split(fileName, constants.SuffixSeparator)
		subnet = parts[0]
		suffix = parts[1]
		newDir := filepath.Join(baseDir, constants.SubnetDir, subnet)
		// instead of checking if it already exists, just let's try to create the dir
		// if it already exists this will not return an error
		if err := os.MkdirAll(newDir, constants.DefaultPerms755); err != nil {
			return err
		}
		fileBytes, err := os.ReadFile(m)
		if err != nil {
			return fmt.Errorf("failed to read original file in `migrateTopLevelFiles` migration: %w", err)
		}
		if err := os.WriteFile(filepath.Join(newDir, suffix), fileBytes, constants.DefaultPerms755); err != nil {
			return fmt.Errorf("failed to write new file in `migrateTopLevelFiles` migration: %w", err)
		}
		if err := os.Remove(m); err != nil {
			return fmt.Errorf("failed to remove original file in `migrateTopLevelFiles` migration: %w", err)
		}
	}
	return nil
}

// Every migration should first check if there are migrations to be run.
// If yes, should run this function to print a message only once
func printMigrationMessage() {
	if showMsg {
		ux.Logger.PrintToUser("This version of the tool will first run an update process for the tool's data directory")
	}
	showMsg = false
	migRunning = true
}

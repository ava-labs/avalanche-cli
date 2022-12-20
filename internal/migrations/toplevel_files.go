// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

// Up to version 1.0.0 the sidecar and genesis files were stored at
// {baseDir} in the top-level.
// Due to new requirements and evolution of the tool, we now store
// every subnet-specific file in {baseDir}/subnets/{subnetName}
func migrateTopLevelFiles(app *application.Avalanche, runner *migrationRunner) error {
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
		runner.printMigrationMessage()
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
		if err := os.Rename(m, filepath.Join(newDir, suffix)); err != nil {
			return fmt.Errorf("failed to move file %s to %s in `migrateTopLevelFiles` migration: %w", m, filepath.Join(newDir, suffix), err)
		}
	}
	return nil
}

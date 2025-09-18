// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

// CopyFile copies a file from src to dest and sets executable permissions.
// Uses utils.FileCopy for the base copy operation, then sets executable permissions.
func CopyFile(src, dest string) error {
	// Use the common file copy logic
	if err := utils.FileCopy(src, dest); err != nil {
		return err
	}

	// Set executable permissions (the difference from utils.FileCopy)
	if err := os.Chmod(dest, constants.DefaultPerms755); err != nil {
		return err
	}

	return nil
}

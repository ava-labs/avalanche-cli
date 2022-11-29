// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
    "fmt"
    "os"
	"crypto/sha256"
	"encoding/hex"
)

func GetSHA256FromDisk(binPath string) (string, error) {
	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("failed looking up plugin binary at %s: %w", binPath, err)
	}
	hasher := sha256.New()
	s, err := os.ReadFile(binPath)
	hasher.Write(s)
	if err != nil {
		return "", fmt.Errorf("failed calculating the sha256 hash of the binary %s: %w", binPath, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}


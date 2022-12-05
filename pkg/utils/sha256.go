// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
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

func SearchSHA256File(file []byte, toSearch string) (string, error) {
	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		sha256Info := strings.Fields(line)
		if len(sha256Info) == 2 {
			if sha256Info[1] == toSearch {
				return sha256Info[0], nil
			}
		}
	}
	return "", fmt.Errorf("%q not found in sha256 file", toSearch)
}

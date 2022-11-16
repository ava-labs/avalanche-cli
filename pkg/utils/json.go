// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// ValidateJSON takes a json string and returns it's byte representation
// if it contains valid JSON
func ValidateJSON(path string) ([]byte, error) {
	var content map[string]interface{}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// if the file is not valid json, this fails
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return nil, fmt.Errorf("this looks like invalid JSON: %w", err)
	}

	return contentBytes, nil
}

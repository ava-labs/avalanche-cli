// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
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

// ReadJSON takes a json string and returns its associated map
// if it contains valid JSON
func ReadJSON(path string) (map[string]interface{}, error) {
	var content map[string]interface{}
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return nil, fmt.Errorf("this looks like invalid JSON: %w", err)
	}
	return content, nil
}

func WriteJSON(path string, data map[string]interface{}) error {
	bs, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bs, constants.WriteReadReadPerms)
}

// Set k=v in JSON string
// e.g., "track-subnets" is the key and value is "a,b,c".
func SetJSONKey(jsonBody string, k string, v interface{}) (string, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(jsonBody), &config); err != nil {
		return "", err
	}
	if v == nil {
		delete(config, k)
	} else {
		config[k] = v
	}
	updatedJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(updatedJSON), nil
}

func GetJSONKey[T any](jsonMap map[string]interface{}, k string) (T, error) {
	intf, ok := jsonMap[k]
	if !ok {
		return *new(T), fmt.Errorf("%w: %s", constants.ErrKeyNotFoundOnMap, k)
	}
	v, ok := intf.(T)
	if !ok {
		return *new(T), fmt.Errorf("unexpected format on %s (%v) on map, expected %T found %T", k, v, new(T), intf)
	}
	return v, nil
}

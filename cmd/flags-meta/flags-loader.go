// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flagsmeta

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

type JSONFlags struct {
	// loaded from JSON
	flagsMap map[string]any
	// for cobra to capture values
	cobraStore map[string]any
}

func LoadCommandFlagsFromJSON(jsonStr string) (*JSONFlags, error) {
	flagsMap := make(map[string]any)
	err := json.Unmarshal([]byte(jsonStr), &flagsMap)
	if err != nil {
		return nil, err
	}

	cobraStore := make(map[string]any)
	return &JSONFlags{
		flagsMap,
		cobraStore,
	}, nil
}

// Load command meta info
func (j *JSONFlags) GetCommandMeta(propName string) (string, error) {
	cmdMetaMap := j.flagsMap["command"].(map[string]any)
	if val, ok := cmdMetaMap[propName].(string); ok {
		return val, nil
	}
	return "", fmt.Errorf("failed to return command meta value: %s", propName)
}

// Load flags config value with given flag name
func GetValue[T any](j *JSONFlags, flagName string) (T, error) {
	if value, ok := j.cobraStore[flagName]; ok {
		if confValue, ok := value.(*T); ok {
			return *confValue, nil
		}
	}
	return *new(T), errors.New("failed to return value")
}

// register flags to cobra command
func (j *JSONFlags) RegisterToCOBRA(
	cmd *cobra.Command,
) error {
	for _, flag := range j.flagsMap["flags"].([]any) {
		flagMap := flag.(map[string]any)
		switch flagMap["type"] {
		case "bool":
			var bval bool
			j.cobraStore[flagMap["var"].(string)] = &bval
			cmd.Flags().BoolVarP(
				&bval,
				flagMap["name"].(string),
				flagMap["shorthand"].(string),
				flagMap["default"].(bool),
				flagMap["usage"].(string),
			)
			// fmt.Printf("registering bool flag: %s\n", flagMap["name"].(string))
		case "string":
			var sval string
			j.cobraStore[flagMap["var"].(string)] = &sval
			cmd.Flags().StringVarP(
				&sval,
				flagMap["name"].(string),
				flagMap["shorthand"].(string),
				flagMap["default"].(string),
				flagMap["usage"].(string),
			)
			// fmt.Printf("registering string flag: %s\n", flagMap["name"].(string))
		case "uint32":
			var u32val uint32
			j.cobraStore[flagMap["var"].(string)] = &u32val
			cmd.Flags().Uint32Var(
				&u32val,
				flagMap["name"].(string),
				uint32(flagMap["default"].(float64 /* JSON number type*/)),
				flagMap["usage"].(string),
			)
			// fmt.Printf("registering uint32 flag: %s\n", flagMap["name"].(string))
		default:
			panic(fmt.Sprintf("unsupported flag type: %T", flag))
		}
	}
	return nil
}

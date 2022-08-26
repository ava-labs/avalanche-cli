// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func EditConfigFile(app *application.Avalanche, subnetID string, networkID string, configFile string, forceWrite bool) error {
	if !forceWrite {
		warn := "This will edit your existing config file. This edit is nondestructive,\n" +
			"but it's always good to have a backup."
		ux.Logger.PrintToUser(warn)
		yes, err := app.Prompt.CaptureYesNo("Proceed?")
		if err != nil {
			return err
		}
		if !yes {
			ux.Logger.PrintToUser("Canceled by user")
			return nil
		}
	}
	fileBytes, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to load avalanchego config file %s: %w", configFile, err)
	}
	if fileBytes == nil {
		fileBytes = []byte("{}")
	}
	var avagoConfig map[string]interface{}
	if err := json.Unmarshal(fileBytes, &avagoConfig); err != nil {
		return fmt.Errorf("failed to unpack the config file %s to JSON: %w", configFile, err)
	}

	// check the old entries in the config file for whitelisted subnets
	oldVal := avagoConfig["whitelisted-subnets"]
	newVal := ""
	if oldVal != nil {
		// if an entry already exists, we check if the subnetID already is part
		// of the whitelisted-subnets...
		exists := false
		var oldValStr string
		var ok bool
		if oldValStr, ok = oldVal.(string); !ok {
			return fmt.Errorf("expected a string value, but got %T", oldVal)
		}
		elems := strings.Split(oldValStr, ",")
		for _, s := range elems {
			if s == subnetID {
				// ...if it is, we just don't need to update the value...
				newVal = oldVal.(string)
				exists = true
			}
		}
		// ...but if it is not, we concatenate the new subnet to the existing ones
		if !exists {
			newVal = strings.Join([]string{oldVal.(string), subnetID}, ",")
		}
	} else {
		// there were no entries yet, so add this subnet as its new value
		newVal = subnetID
	}
	avagoConfig["whitelisted-subnets"] = newVal
	avagoConfig["network-id"] = networkID

	writeBytes, err := json.MarshalIndent(avagoConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to pack JSON to bytes for the config file: %w", err)
	}
	if err := os.WriteFile(configFile, writeBytes, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("failed to write JSON config file, check permissions? %w", err)
	}
	msg := `The config file has been edited. To use it, make sure to start the node with the '--config-file' option, e.g.

./build/avalanchego --config-file %s

(using your binary location). The node has to be restarted for the changes to take effect.`
	ux.Logger.PrintToUser(msg, configFile)
	return nil
}

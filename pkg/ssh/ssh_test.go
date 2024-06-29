// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func TestReplaceCustomVarDashboardValues(t *testing.T) {
	tmpDir := os.TempDir()
	testDir, err := os.MkdirTemp(tmpDir, "dashboard-test")
	if err != nil {
		t.Fatalf("Error creating test dir: %v", err)
	}
	tempFileName := filepath.Join(testDir, "test_dashboard.json")
	tempContent := []byte(`{
		"templating": {
			"list": [
				{
					"current": {
						"selected": true,
						"text": "CHAIN_ID_VAL",
						"value": "CHAIN_ID_VAL"
					},
					"hide": 0,
					"includeAll": false,
					"multi": false,
					"name": "CHAIN_ID",
					"options": [
						{
							"selected": true,
							"text": "CHAIN_ID_VAL",
							"value": "CHAIN_ID_VAL"
						}
					],
					"query": "CHAIN_ID_VAL",
					"queryValue": "",
					"skipUrlSync": false,
					"type": "custom"
				}
			]
		}
	}`)
	err = os.WriteFile(tempFileName, tempContent, constants.WriteReadUserOnlyPerms)
	if err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}
	defer func() {
		err := os.WriteFile(tempFileName, []byte{}, constants.WriteReadUserOnlyPerms)
		if err != nil {
			t.Fatalf("Error cleaning up test file: %v", err)
		}
	}()

	err = replaceCustomVarDashboardValues(tempFileName, "newChainID")
	if err != nil {
		t.Fatalf("Error replacing custom variables: %v", err)
	}
	modifiedContent, err := os.ReadFile(tempFileName)
	if err != nil {
		t.Fatalf("Error reading modified content: %v", err)
	}

	expectedContent := `{
		"templating": {
			"list": [
				{
					"current": {
						"selected": true,
						"text": "newChainID",
						"value": "newChainID"
					},
					"hide": 0,
					"includeAll": false,
					"multi": false,
					"name": "CHAIN_ID",
					"options": [
						{
							"selected": true,
							"text": "newChainID",
							"value": "newChainID"
						}
					],
					"query": "newChainID",
					"queryValue": "",
					"skipUrlSync": false,
					"type": "custom"
				}
			]
		}
	}`
	if string(modifiedContent) != expectedContent {
		t.Errorf("Expected content after replacement:\n%s\nGot:\n%s", expectedContent, string(modifiedContent))
	}
}

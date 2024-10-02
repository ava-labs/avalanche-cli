// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/stretchr/testify/require"
)

func TestReplaceCustomVarDashboardValues(t *testing.T) {
	tmpDir := os.TempDir()
	testDir, err := os.MkdirTemp(tmpDir, "dashboard-test")
	require.NoError(t, err)
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
	require.NoError(t, os.WriteFile(tempFileName, tempContent, constants.WriteReadUserOnlyPerms))
	defer func() {
		require.NoError(t, os.WriteFile(tempFileName, []byte{}, constants.WriteReadUserOnlyPerms))
	}()

	err = replaceCustomVarDashboardValues(tempFileName, "newChainID")
	require.NoError(t, err)

	modifiedContent, err := os.ReadFile(tempFileName)
	require.NoError(t, err)

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
	require.Equal(t, expectedContent, string(modifiedContent))
}

package apmintegration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGithubOrg(t *testing.T) {
	type test struct {
		name        string
		url         string
		expectedOrg string
		expectedErr bool
	}

	tests := []test{
		{
			name:        "Success",
			url:         "https://github.com/ava-labs/avalanche-plugins-core.git",
			expectedOrg: "ava-labs",
			expectedErr: false,
		},
		{
			name:        "No org",
			url:         "avalanche-plugins-core",
			expectedOrg: "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			org, err := getGithubOrg(tt.url)
			assert.Equal(tt.expectedOrg, org)
			if tt.expectedErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestGetGithubRepo(t *testing.T) {
	type test struct {
		name         string
		url          string
		expectedRepo string
		expectedErr  bool
	}

	tests := []test{
		{
			name:         "Success",
			url:          "https://github.com/ava-labs/avalanche-plugins-core.git",
			expectedRepo: "avalanche-plugins-core",
			expectedErr:  false,
		},
		{
			name:         "No repo",
			url:          "avalanche-plugins-core",
			expectedRepo: "",
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			repo, err := getGithubRepo(tt.url)
			assert.Equal(tt.expectedRepo, repo)
			if tt.expectedErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestGetAlias(t *testing.T) {
	assert := assert.New(t)

	url := "https://github.com/ava-labs/avalanche-plugins-core.git"
	expectedAlias := "ava-labs/avalanche-plugins-core"

	alias, err := getAlias(url)
	assert.NoError(err)
	assert.Equal(expectedAlias, alias)
}

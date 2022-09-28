package upgradecmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAtMostOneNetworkSelected(t *testing.T) {
	assert := assert.New(t)

	type test struct {
		name       string
		useFuture  bool
		useLocal   bool
		useFuji    bool
		useMainnet bool
		valid      bool
	}

	tests := []test{
		{
			name:       "all false",
			useFuture:  false,
			useLocal:   false,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "future true",
			useFuture:  true,
			useLocal:   false,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "local true",
			useFuture:  false,
			useLocal:   true,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "fuji true",
			useFuture:  false,
			useLocal:   false,
			useFuji:    true,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "mainnet true",
			useFuture:  false,
			useLocal:   false,
			useFuji:    false,
			useMainnet: true,
			valid:      true,
		},
		{
			name:       "double true 1",
			useFuture:  true,
			useLocal:   true,
			useFuji:    false,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 2",
			useFuture:  true,
			useLocal:   false,
			useFuji:    true,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 3",
			useFuture:  true,
			useLocal:   false,
			useFuji:    false,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "double true 4",
			useFuture:  false,
			useLocal:   true,
			useFuji:    true,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 5",
			useFuture:  false,
			useLocal:   true,
			useFuji:    false,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "double true 6",
			useFuture:  false,
			useLocal:   false,
			useFuji:    true,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "all true",
			useFuture:  true,
			useLocal:   true,
			useFuji:    true,
			useMainnet: true,
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFuture = tt.useFuture
			useLocal = tt.useLocal
			useFuji = tt.useFuji
			useMainnet = tt.useMainnet

			accepted := atMostOneNetworkSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

func TestAtMostOneVersionSelected(t *testing.T) {
	assert := assert.New(t)

	type test struct {
		name      string
		useLatest bool
		version   string
		binary    string
		valid     bool
	}

	tests := []test{
		{
			name:      "all empty",
			useLatest: false,
			version:   "",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 1",
			useLatest: true,
			version:   "",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 2",
			useLatest: false,
			version:   "v1.2.0",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 3",
			useLatest: false,
			version:   "",
			binary:    "home",
			valid:     true,
		},
		{
			name:      "two selected 1",
			useLatest: true,
			version:   "v1.2.0",
			binary:    "",
			valid:     false,
		},
		{
			name:      "two selected 2",
			useLatest: true,
			version:   "",
			binary:    "home",
			valid:     false,
		},
		{
			name:      "two selected 3",
			useLatest: false,
			version:   "v1.2.0",
			binary:    "home",
			valid:     false,
		},
		{
			name:      "all selected",
			useLatest: true,
			version:   "v1.2.0",
			binary:    "home",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useLatest = tt.useLatest
			targetVersion = tt.version
			useBinary = tt.binary

			accepted := atMostOneVersionSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

func TestAtMostOneAutomationSelected(t *testing.T) {
	assert := assert.New(t)

	type test struct {
		name      string
		useManual bool
		pluginDir string
		valid     bool
	}

	tests := []test{
		{
			name:      "all empty",
			useManual: false,
			pluginDir: "",
			valid:     true,
		},
		{
			name:      "manual selected",
			useManual: true,
			pluginDir: "",
			valid:     true,
		},
		{
			name:      "auto selected",
			useManual: false,
			pluginDir: "home",
			valid:     true,
		},
		{
			name:      "both selected",
			useManual: true,
			pluginDir: "home",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useManual = tt.useManual
			pluginDir = tt.pluginDir

			accepted := atMostOneAutomationSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

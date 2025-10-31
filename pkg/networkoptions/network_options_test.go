// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkoptions

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestNetworkToNetworkFlags(t *testing.T) {
	tests := []struct {
		name     string
		network  models.Network
		expected NetworkFlags
	}{
		{
			name: "Local Network",
			network: models.Network{
				Kind:        models.Local,
				ID:          1337,
				Endpoint:    "http://127.0.0.1:9650",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    true,
				UseDevnet:   false,
				UseFuji:     false,
				UseMainnet:  false,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Fuji Network",
			network: models.Network{
				Kind:        models.Fuji,
				ID:          5,
				Endpoint:    "https://api.avax-test.network",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   false,
				UseFuji:     true,
				UseMainnet:  false,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Mainnet Network",
			network: models.Network{
				Kind:        models.Mainnet,
				ID:          1,
				Endpoint:    "https://api.avax.network",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   false,
				UseFuji:     false,
				UseMainnet:  true,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Devnet Network",
			network: models.Network{
				Kind:        models.Devnet,
				ID:          1338,
				Endpoint:    "https://custom-devnet.example.com",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   true,
				UseFuji:     false,
				UseMainnet:  false,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Granite Network",
			network: models.Network{
				Kind:        models.Granite,
				ID:          76,
				Endpoint:    "https://granite.avax-dev.network",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   false,
				UseFuji:     false,
				UseMainnet:  false,
				UseGranite:  true,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Cluster Network",
			network: models.Network{
				Kind:        models.Devnet,
				ID:          1338,
				Endpoint:    "https://cluster.example.com",
				ClusterName: "my-cluster",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   true,
				UseFuji:     false,
				UseMainnet:  false,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
		{
			name: "Undefined Network",
			network: models.Network{
				Kind:        models.Undefined,
				ID:          0,
				Endpoint:    "",
				ClusterName: "",
			},
			expected: NetworkFlags{
				UseLocal:    false,
				UseDevnet:   false,
				UseFuji:     false,
				UseMainnet:  false,
				UseGranite:  false,
				Endpoint:    "",
				ClusterName: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NetworkToNetworkFlags(tt.network)
			require.Equal(t, tt.expected, result)
		})
	}
}

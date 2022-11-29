// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/semver"
)

type testContext struct {
	expected    map[string]string
	sourceEVM   string
	spacesVM    string
	sourceAvago string
	shouldFail  bool
}

type testMapper struct {
	app            *application.Avalanche
	currentContext *testContext
	srv            *httptest.Server
	t              *testing.T
}

func newTestMapper(t *testing.T) *testMapper {
	app := &application.Avalanche{
		Downloader: application.NewDownloader(),
		Log:        logging.NoLog{},
	}
	return &testMapper{
		app,
		nil,
		nil,
		t,
	}
}

func (m *testMapper) GetLatestAvagoByProtoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error) {
	cBytes := []byte(m.currentContext.sourceAvago)

	var compat models.AvagoCompatiblity
	if err := json.Unmarshal(cBytes, &compat); err != nil {
		return "", err
	}
	vers := compat[strconv.Itoa(rpcVersion)]
	if len(vers) == 0 {
		return "", errors.New("test zero length versions")
	}
	if len(vers) > 1 {
		semver.Sort(vers)
	}
	// take the last
	return vers[len(vers)-1], nil
}

func (m *testMapper) getVersionMapping(tc *testContext) (map[string]string, error) {
	mapping = nil
	m.currentContext = tc
	return GetVersionMapping(m)
}

func (m *testMapper) GetApp() *application.Avalanche {
	return m.app
}

func (m *testMapper) GetCompatURL(vmType models.VMType) string {
	switch vmType {
	case models.SubnetEvm:
		return m.srv.URL + "/evm"
	case models.SpacesVM:
		return m.srv.URL + "/spaces"
	default:
		m.t.Fatalf("unexpected vmType: %T", vmType)
	}
	return ""
}

func (m *testMapper) GetAvagoURL() string {
	return m.srv.URL + "/avago"
}

func (m *testMapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var err error
	switch r.URL.Path {
	case "/evm":
		_, err = w.Write([]byte(m.currentContext.sourceEVM))
	case "/spaces":
		_, err = w.Write([]byte(m.currentContext.spacesVM))
	case "/avago":
		_, err = w.Write([]byte(m.currentContext.sourceAvago))
	default:
		m.t.Fatalf("Unexpected path URL for test server: %s\n", r.URL.Path)
	}
	if err != nil {
		m.t.Fatal(err)
	}
}

func TestGetVersionMapping(t *testing.T) {
	assert := assert.New(t)
	m := newTestMapper(t)
	srv := httptest.NewServer(m)
	defer srv.Close()
	m.srv = srv

	testContexts := []*testContext{
		{
			shouldFail: false,
			expected: map[string]string{
				SoloSubnetEVMKey1:      "v0.4.2",
				SoloSubnetEVMKey2:      "v0.4.1",
				SoloAvagoKey:           "v1.9.1",
				OnlyAvagoKey:           OnlyAvagoValue,
				MultiAvago1Key:         "v1.9.3",
				MultiAvago2Key:         "v1.9.2",
				MultiAvagoSubnetEVMKey: "v0.4.3",
				LatestEVM2AvagoKey:     "v0.4.3",
				LatestAvago2EVMKey:     "v1.9.3",
				Spaces2AvagoKey:        "v0.0.12",
				Avago2SpacesKey:        "v1.9.3",
			},
			sourceEVM: `{
						"rpcChainVMProtocolVersion": {
							"v0.4.4": 19,
							"v0.4.3": 19,
							"v0.4.2": 18,
							"v0.4.1": 18,
							"v0.4.0": 17
						}
				  }`,
			spacesVM: `{
  					"rpcChainVMProtocolVersion": {
    					"v0.0.12": 19,
    					"v0.0.11": 19,
    					"v0.0.10": 19,
    					"v0.0.9": 17,
    					"v0.0.8": 16,
    					"v0.0.7": 15
						}
					}`,
			sourceAvago: `{
						"19": [
							"v1.9.2",
							"v1.9.3"
						],
						"18": [
							"v1.9.1"
						],
						"17": [
							"v1.9.0"
						]
				  }`,
		},
		{
			shouldFail: false,
			expected: map[string]string{
				SoloSubnetEVMKey1:      "v0.9.9",
				SoloSubnetEVMKey2:      "v0.9.8",
				SoloAvagoKey:           "v2.3.4",
				OnlyAvagoKey:           OnlyAvagoValue,
				MultiAvago1Key:         "v2.3.4",
				MultiAvago2Key:         "v2.3.3",
				MultiAvagoSubnetEVMKey: "v0.9.9",
				LatestEVM2AvagoKey:     "v0.9.9",
				LatestAvago2EVMKey:     "v2.3.4",
				Spaces2AvagoKey:        "v4.5.12",
				Avago2SpacesKey:        "v2.3.4",
			},
			sourceEVM: `{
					"rpcChainVMProtocolVersion": {
						"v1.0.0": 100,
						"v0.9.9": 99,
						"v0.9.8": 99,
						"v0.4.2": 18,
						"v0.4.1": 18,
						"v0.4.0": 17
					}
			  }`,
			spacesVM: `{
  					"rpcChainVMProtocolVersion": {
    					"v4.5.12": 99,
    					"v3.2.12": 77,
    					"v2.1.11": 66,
    					"v0.0.10": 19
						}
					}`,
			sourceAvago: `{
					"99": [
						"v2.3.4",
						"v2.3.3"
					],
					"18": [
						"v1.9.1"
					],
					"17": [
						"v1.9.0"
					]
			  }`,
		},
		{
			shouldFail: false,
			expected: map[string]string{
				SoloSubnetEVMKey1:      "v0.4.2",
				SoloSubnetEVMKey2:      "v0.4.1",
				SoloAvagoKey:           "v2.1.1",
				OnlyAvagoKey:           OnlyAvagoValue,
				MultiAvago1Key:         "v2.1.1",
				MultiAvago2Key:         "v2.1.0",
				MultiAvagoSubnetEVMKey: "v0.4.2",
				LatestEVM2AvagoKey:     "v0.9.9",
				LatestAvago2EVMKey:     "v4.3.2",
				Spaces2AvagoKey:        "v3.2.12",
				Avago2SpacesKey:        "v2.1.1",
			},
			sourceEVM: `{
					"rpcChainVMProtocolVersion": {
						"v1.0.0": 100,
						"v0.9.9": 99,
						"v0.5.3": 88,
						"v0.5.2": 87,
						"v0.5.1": 86,
						"v0.4.2": 77,
						"v0.4.1": 77,
						"v0.4.0": 17
					}
			  }`,
			spacesVM: `{
  					"rpcChainVMProtocolVersion": {
    					"v3.2.12": 77,
    					"v2.1.11": 66,
    					"v0.0.10": 19
						}
					}`,
			sourceAvago: `{
					"99": [
						"v4.3.2"
					],
					"88": [
						"v2.3.4"
					],
					"87": [
						"v2.2.2"
					],
					"86": [
						"v2.2.1"
					],
					"77": [
						"v2.1.1",
						"v2.1.0"
					],
					"18": [
						"v1.9.1"
					],
					"17": [
						"v1.9.0"
					]
			  }`,
		},
		{
			shouldFail:  true,
			expected:    map[string]string{},
			sourceEVM:   `{}`,
			sourceAvago: `{}`,
		},
		{
			shouldFail: true,
			expected:   map[string]string{},
			sourceAvago: `{
					"99": [
						"v2.3.4",
						"v2.3.3"
					],
					"88": [
						"v1.9.1"
					],
					"77": [
						"v1.9.0"
					]
			  }`,
			sourceEVM: `{
					"rpcChainVMProtocolVersion": {
						"v1.0.0": 100,
						"v0.9.9": 66,
						"v0.9.8": 55,
						"v0.4.2": 44,
						"v0.4.1": 33,
						"v0.4.0": 22
					}
			  }`,
		},
		{
			shouldFail:  true,
			expected:    map[string]string{},
			sourceAvago: `{}`,
			sourceEVM: `{
					"rpcChainVMProtocolVersion": {
						"v1.0.0": 100,
						"v0.9.9": 99,
						"v0.9.8": 99,
						"v0.4.2": 18,
						"v0.4.1": 18,
						"v0.4.0": 17
					}
			  }`,
		},
		{
			shouldFail: true,
			expected:   map[string]string{},
			sourceEVM:  `{}`,
			sourceAvago: `{
					"99": [
						"v2.3.4",
						"v2.3.3"
					],
					"18": [
						"v1.9.1"
					],
					"17": [
						"v1.9.0"
					]
			  }`,
		},
	}

	for i, tc := range testContexts {
		mapping, err := m.getVersionMapping(tc)
		if tc.shouldFail {
			assert.Error(err)
			continue
		} else {
			assert.NoError(err)
		}
		assert.Equal(tc.expected, mapping, fmt.Sprintf("iteration: %d", i))
	}
}

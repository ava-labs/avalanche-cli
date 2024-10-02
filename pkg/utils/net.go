// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
)

// GetUserIPAddress retrieves the IP address of the user.
func GetUserIPAddress() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("HTTP request failed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	ipAddress, ok := result["ip"].(string)
	if ok {
		if net.ParseIP(ipAddress) == nil {
			return "", errors.New("invalid IP address")
		}
		return ipAddress, nil
	}

	return "", errors.New("no IP address found")
}

func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

// IsValidURL checks if a URL is valid.
func IsValidURL(urlString string) bool {
	u, err := url.Parse(urlString)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}

func StringSliceToNodeIds(s []string) ([]ids.NodeID, error) {
	nodeIDs := []ids.NodeID{}
	for _, id := range s {
		id = strings.Trim(id, " ")
		if id == "" {
			continue
		}
		nodeId, err := ids.NodeIDFromString(id)
		if err != nil {
			return nil, err
		}
		nodeIDs = append(nodeIDs, nodeId)
	}
	return nodeIDs, nil
}

func StringSliceToNetipPorts(s []string) ([]netip.AddrPort, error) {
	var nodeIds []netip.AddrPort
	for _, id := range s {
		id = strings.Trim(id, " ")
		if id == "" {
			continue
		}
		addrPort, err := netip.ParseAddrPort(id)
		if err != nil {
			return nil, err
		}
		nodeIds = append(nodeIds, addrPort)
	}
	return nodeIds, nil
}

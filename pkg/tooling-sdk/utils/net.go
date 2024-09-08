// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

// Get the host and port from a URI. The URI should be in the format http://host:port or https://host:port or host:port
func GetURIHostAndPort(uri string) (string, uint32, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse uri %s: %w", uri, err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host/port at uri %s: %w", uri, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to convert port to uint at uri %s: %w", uri, err)
	}
	return host, uint32(port), nil
}

func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

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

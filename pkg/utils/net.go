// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-tooling-sdk-go/utils"
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

// IsValidIPPort checks if an  string IP:port pair is valid.
func IsValidIPPort(ipPortPair string) bool {
	if _, err := GetIPPort(ipPortPair); err != nil {
		return false
	}
	return true
}

// GetIPPort parses netip.IPPort from string that also may include http schema
func GetIPPort(uri string) (netip.AddrPort, error) {
	uri = strings.TrimPrefix(uri, "https://")
	uri = strings.TrimPrefix(uri, "http://")
	return netip.ParseAddrPort(uri)
}

// SplitRPCURI splits the RPC URI into `endpoint` and `chain`.
// Reverse operation of `fmt.Sprintf("%s/ext/bc/%s", endpoint, chain)`.
// returns the `uri` and `chain` as strings, or an error if the request URI is invalid.
func SplitAvalanchegoRPCURI(requestURI string) (string, string, error) {
	// Define the regex pattern
	pattern := `^(https?://[^/]+)/ext/bc/([^/]+)/rpc$`
	regex := regexp.MustCompile(pattern)

	// Match the pattern
	matches := regex.FindStringSubmatch(requestURI)
	if matches == nil || len(matches) != 3 {
		return "", "", fmt.Errorf("invalid request URI format")
	}

	// Extract `endpoint` and `chain`
	endpoint := matches[1]
	chain := matches[2]

	return endpoint, chain, nil
}

// IsPortAvailable checks if a port is available for binding.
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		// If we can't connect, the port is available
		return true
	}
	// If we can connect, the port is in use
	_ = conn.Close()
	return false
}

// FindAvailablePort finds an available port starting from the given port.
// It tries the given port first, then increments by step until finding an available port.
// Returns an error if no port is found within the timeout.
func FindAvailablePort(startPort, step int, timeout time.Duration) (int, error) {
	ctx, cancel := utils.GetTimedContext(timeout)
	defer cancel()

	port := startPort
	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout while searching for available port starting from %d: %w", startPort, ctx.Err())
		default:
			if IsPortAvailable(port) {
				return port, nil
			}
			port += step
		}
	}
}

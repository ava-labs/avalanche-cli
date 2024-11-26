// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
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
	"strings"
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
	if _, err := netip.ParseAddrPort(ipPortPair); err != nil {
		return false
	}
	return true
}

// SplitRPCURI splits the RPC URI into `endpoint` and `chain`.
// Reverse operation of `fmt.Sprintf("%s/ext/bc/%s", endpoint, chain)`.
// returns the `uri` and `chain` as strings, or an error if the request URI is invalid.
func SplitRPCURI(requestUri string) (string, string, error) {
	// Check if the request URI contains "/ext/bc/"
	splitPoint := "/ext/bc/"
	index := strings.Index(requestUri, splitPoint)
	if index == -1 {
		return "", "", fmt.Errorf("invalid request URI format")
	}
	// Extract `uri` and `chain`
	uri := requestUri[:index]
	chain := requestUri[index+len(splitPoint):]
	return uri, strings.TrimSuffix(chain, "/"), nil
}

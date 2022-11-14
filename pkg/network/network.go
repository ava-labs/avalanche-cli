// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/api/info"
)

type StatusChecker interface {
	GetCurrentNetworkVersion() (string, int, error)
}

type networkStatusChecker struct{}

func NewStatusChecker() StatusChecker {
	return networkStatusChecker{}
}

func (networkStatusChecker) GetCurrentNetworkVersion() (string, int, error) {
	ctx := context.Background()
	infoClient := info.NewClient(constants.LocalAPIEndpoint)
	versionResponse, err := infoClient.GetNodeVersion(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("unable to determine rpc version: %w", err)
	}

	// version is in format avalanche/x.y.z, need to turn to semantic
	splitVersion := strings.Split(versionResponse.Version, "/")
	if len(splitVersion) != 2 {
		return "", 0, errors.New("unable to parse avalanchego version " + versionResponse.Version)
	}
	// index 0 should be avalanche, index 1 will be version
	parsedVersion := "v" + splitVersion[1]

	return parsedVersion, int(versionResponse.RPCProtocolVersion), nil
}

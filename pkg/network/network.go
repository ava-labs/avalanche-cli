package network

import (
	"context"
	"fmt"

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

	return versionResponse.Version, int(versionResponse.RPCProtocolVersion), nil
}

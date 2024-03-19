// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkoptions

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type NetworkOption int64

const (
	Undefined NetworkOption = iota
	Mainnet
	Fuji
	Local
	Devnet
	Cluster
)

func (n NetworkOption) String() string {
	switch n {
	case Mainnet:
		return "Mainnet"
	case Fuji:
		return "Fuji"
	case Local:
		return "Local Network"
	case Devnet:
		return "Devnet"
	case Cluster:
		return "Cluster"
	}
	return "invalid network"
}

func networkOptionFromString(s string) NetworkOption {
	switch s {
	case "Mainnet":
		return Mainnet
	case "Fuji":
		return Fuji
	case "Local Network":
		return Local
	case "Devnet":
		return Devnet
	case "Cluster":
		return Cluster
	}
	return Undefined
}

type NetworkFlags struct {
	UseLocal    bool
	UseDevnet   bool
	UseFuji     bool
	UseMainnet  bool
	Endpoint    string
	ClusterName string
}

func AddNetworkFlagsToCmd(cmd *cobra.Command, networkFlags *NetworkFlags, alwaysAddEndpoint bool, supportedNetworkOptions []NetworkOption) {
	addEndpoint := alwaysAddEndpoint
	for _, networkOption := range supportedNetworkOptions {
		switch networkOption {
		case Local:
			cmd.Flags().BoolVarP(&networkFlags.UseLocal, "local", "l", false, "opeate on a local network")
		case Devnet:
			cmd.Flags().BoolVar(&networkFlags.UseDevnet, "devnet", false, "operate on a devnet network")
			addEndpoint = true
		case Fuji:
			cmd.Flags().BoolVarP(&networkFlags.UseFuji, "testnet", "t", false, "operate on testnet (alias to `fuji`)")
			cmd.Flags().BoolVarP(&networkFlags.UseFuji, "fuji", "f", false, "operate on fuji (alias to `testnet`")
		case Mainnet:
			cmd.Flags().BoolVarP(&networkFlags.UseMainnet, "mainnet", "m", false, "operate on mainnet")
		case Cluster:
			cmd.Flags().StringVar(&networkFlags.ClusterName, "cluster", "", "operate on the given cluster")
		}
	}
	if addEndpoint {
		cmd.Flags().StringVar(&networkFlags.Endpoint, "endpoint", "", "use the given endpoint for network operations")
	}
}

func GetSupportedNetworkOptionsForSubnet(
	app *application.Avalanche,
	subnetName string,
	supportedNetworkOptions []NetworkOption,
) ([]NetworkOption, []string, []string, error) {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return nil, nil, nil, err
	}
	filteredSupportedNetworkOptions := []NetworkOption{}
	for _, networkOption := range supportedNetworkOptions {
		isInSidecar := false
		for networkName := range sc.Networks {
			if strings.HasPrefix(networkName, networkOption.String()) {
				isInSidecar = true
			}
			if os.Getenv(constants.SimulatePublicNetwork) != "" {
				if strings.HasPrefix(networkName, Local.String()) {
					if networkOption == Fuji || networkOption == Mainnet {
						isInSidecar = true
					}
				}
			}
		}
		if isInSidecar {
			filteredSupportedNetworkOptions = append(filteredSupportedNetworkOptions, networkOption)
		}
	}
	supportsClusters := false
	if _, err := utils.GetIndexInSlice(filteredSupportedNetworkOptions, Cluster); err == nil {
		supportsClusters = true
	}
	supportsDevnets := false
	if _, err := utils.GetIndexInSlice(filteredSupportedNetworkOptions, Devnet); err == nil {
		supportsDevnets = true
	}
	clusterNames := []string{}
	devnetEndpoints := []string{}
	for networkName := range sc.Networks {
		if supportsClusters && strings.HasPrefix(networkName, Cluster.String()) {
			parts := strings.Split(networkName, " ")
			if len(parts) != 2 {
				return nil, nil, nil, fmt.Errorf("expected 'Cluster clusterName' on network name %s", networkName)
			}
			clusterNames = append(clusterNames, parts[1])
		}
		if supportsDevnets && strings.HasPrefix(networkName, Devnet.String()) {
			parts := strings.Split(networkName, " ")
			if len(parts) > 2 {
				return nil, nil, nil, fmt.Errorf("expected 'Devnet endpoint' on network name %s", networkName)
			}
			if len(parts) == 2 {
				endpoint := parts[1]
				devnetEndpoints = append(devnetEndpoints, endpoint)
			}
		}
	}
	return filteredSupportedNetworkOptions, clusterNames, devnetEndpoints, nil
}

func GetNetworkFromCmdLineFlags(
	app *application.Avalanche,
	networkFlags NetworkFlags,
	requireDevnetEndpointSpecification bool,
	supportedNetworkOptions []NetworkOption,
	subnetName string,
) (models.Network, error) {
	var err error
	supportedNetworkOptionsStrs := ""
	filteredSupportedNetworkOptionsStrs := ""
	scClusterNames := []string{}
	scDevnetEndpoints := []string{}
	if subnetName != "" {
		var filteredSupportedNetworkOptions []NetworkOption
		filteredSupportedNetworkOptions, scClusterNames, scDevnetEndpoints, err = GetSupportedNetworkOptionsForSubnet(app, subnetName, supportedNetworkOptions)
		if err != nil {
			return models.UndefinedNetwork, err
		}
		supportedNetworkOptionsStrs = strings.Join(utils.Map(supportedNetworkOptions, func(s NetworkOption) string { return s.String() }), ", ")
		filteredSupportedNetworkOptionsStrs = strings.Join(utils.Map(filteredSupportedNetworkOptions, func(s NetworkOption) string { return s.String() }), ", ")
		if len(filteredSupportedNetworkOptions) == 0 {
			return models.UndefinedNetwork, fmt.Errorf("no supported deployed networks available on subnet %q. please deploy to one of: [%s]", subnetName, supportedNetworkOptionsStrs)
		}
		supportedNetworkOptions = filteredSupportedNetworkOptions
	}
	// supported flags
	networkFlagsMap := map[NetworkOption]string{
		Local:   "--local",
		Devnet:  "--devnet",
		Fuji:    "--fuji/--testnet",
		Mainnet: "--mainnet",
		Cluster: "--cluster",
	}
	supportedNetworksFlags := strings.Join(utils.Map(supportedNetworkOptions, func(n NetworkOption) string { return networkFlagsMap[n] }), ", ")
	// received option
	networkOption := Undefined
	switch {
	case networkFlags.UseLocal:
		networkOption = Local
	case networkFlags.UseDevnet:
		networkOption = Devnet
	case networkFlags.UseFuji:
		networkOption = Fuji
	case networkFlags.UseMainnet:
		networkOption = Mainnet
	case networkFlags.ClusterName != "":
		networkOption = Cluster
	}
	// unsupported option
	if networkOption != Undefined && !slices.Contains(supportedNetworkOptions, networkOption) {
		errMsg := fmt.Errorf("network flag %s is not supported. use one of %s", networkFlagsMap[networkOption], supportedNetworksFlags)
		if subnetName != "" {
			clustersMsg := ""
			endpointsMsg := ""
			if len(scClusterNames) != 0 {
				clustersMsg = fmt.Sprintf(". valid clusters: [%s]", strings.Join(scClusterNames, ", "))
			}
			if len(scDevnetEndpoints) != 0 {
				endpointsMsg = fmt.Sprintf(". valid devnet endpoints: [%s]", strings.Join(scDevnetEndpoints, ", "))
			}
			errMsg = fmt.Errorf("network flag %s is not available on subnet %s. use one of %s or made a deploy for that network%s%s", networkFlagsMap[networkOption], subnetName, supportedNetworksFlags, clustersMsg, endpointsMsg)
		}
		return models.UndefinedNetwork, errMsg
	}
	// mutual exclusion
	if !flags.EnsureMutuallyExclusive([]bool{networkFlags.UseLocal, networkFlags.UseDevnet, networkFlags.UseFuji, networkFlags.UseMainnet, networkFlags.ClusterName != ""}) {
		return models.UndefinedNetwork, fmt.Errorf("network flags %s are mutually exclusive", supportedNetworksFlags)
	}

	if networkOption == Undefined {
		if subnetName != "" && supportedNetworkOptionsStrs != filteredSupportedNetworkOptionsStrs {
			ux.Logger.PrintToUser("currently supported deployed networks on %q for this command: [%s]", subnetName, filteredSupportedNetworkOptionsStrs)
			ux.Logger.PrintToUser("for more options, deploy %q to one of: [%s]", subnetName, supportedNetworkOptionsStrs)
			ux.Logger.PrintToUser("")
		}
		// undefined, so prompt
		clusterNames, err := app.ListClusterNames()
		if err != nil {
			return models.UndefinedNetwork, err
		}
		if subnetName != "" {
			clusterNames = scClusterNames
		}
		if len(clusterNames) == 0 {
			if index, err := utils.GetIndexInSlice(supportedNetworkOptions, Cluster); err == nil {
				supportedNetworkOptions = append(supportedNetworkOptions[:index], supportedNetworkOptions[index+1:]...)
			}
		}
		networkOptionStr, err := app.Prompt.CaptureList(
			"Choose a network for the operation",
			utils.Map(supportedNetworkOptions, func(n NetworkOption) string { return n.String() }),
		)
		if err != nil {
			return models.UndefinedNetwork, err
		}
		networkOption = networkOptionFromString(networkOptionStr)
		if networkOption == Cluster {
			networkFlags.ClusterName, err = app.Prompt.CaptureList(
				"Choose a cluster",
				clusterNames,
			)
			if err != nil {
				return models.UndefinedNetwork, err
			}
		}
	}

	if networkOption == Devnet && networkFlags.Endpoint == "" && requireDevnetEndpointSpecification {
		if len(scDevnetEndpoints) != 0 {
			networkFlags.Endpoint, err = app.Prompt.CaptureList(
				"Choose an endpoint",
				scDevnetEndpoints,
			)
			if err != nil {
				return models.UndefinedNetwork, err
			}
		} else {
			networkFlags.Endpoint, err = app.Prompt.CaptureURL(fmt.Sprintf("%s Network Endpoint", networkOption.String()), false)
			if err != nil {
				return models.UndefinedNetwork, err
			}
		}
	}

	network := models.UndefinedNetwork
	switch networkOption {
	case Local:
		network = models.NewLocalNetwork()
	case Devnet:
		networkID := uint32(0)
		if networkFlags.Endpoint != "" {
			infoClient := info.NewClient(networkFlags.Endpoint)
			ctx, cancel := utils.GetAPIContext()
			defer cancel()
			networkID, err = infoClient.GetNetworkID(ctx)
			if err != nil {
				return models.UndefinedNetwork, err
			}
		}
		network = models.NewDevnetNetwork(networkFlags.Endpoint, networkID)
	case Fuji:
		network = models.NewFujiNetwork()
	case Mainnet:
		network = models.NewMainnetNetwork()
	case Cluster:
		network, err = app.GetClusterNetwork(networkFlags.ClusterName)
		if err != nil {
			return models.UndefinedNetwork, err
		}
	}
	// on all cases, enable user setting specific endpoint
	if networkFlags.Endpoint != "" {
		network.Endpoint = networkFlags.Endpoint
	}

	return network, nil
}

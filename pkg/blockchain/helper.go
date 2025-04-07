// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/network/peer"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func GetAggregatorExtraPeers(
	app *application.Avalanche,
	clusterName string,
	extraURIs []string,
) ([]info.Peer, error) {
	uris, err := GetAggregatorNetworkUris(app, clusterName)
	if err != nil {
		return nil, err
	}
	uris = append(uris, extraURIs...)
	urisSet := set.Of(uris...)
	uris = urisSet.List()
	return UrisToPeers(uris)
}

func GetAggregatorNetworkUris(app *application.Avalanche, clusterName string) ([]string, error) {
	aggregatorExtraPeerEndpointsUris := []string{}
	if clusterName != "" {
		if localnet.LocalClusterExists(app, clusterName) {
			return localnet.GetLocalClusterURIs(app, clusterName)
		} else { // remote cluster case
			clustersConfig, err := app.LoadClustersConfig()
			if err != nil {
				return nil, err
			}
			clusterConfig := clustersConfig.Clusters[clusterName]
			hostIDs := utils.Filter(clusterConfig.GetCloudIDs(), clusterConfig.IsAvalancheGoHost)
			for _, hostID := range hostIDs {
				if nodeConfig, err := app.LoadClusterNodeConfig(hostID); err != nil {
					return nil, err
				} else {
					aggregatorExtraPeerEndpointsUris = append(aggregatorExtraPeerEndpointsUris, fmt.Sprintf("http://%s:%d", nodeConfig.ElasticIP, constants.AvalancheGoAPIPort))
				}
			}
		}
	}
	return aggregatorExtraPeerEndpointsUris, nil
}

func UrisToPeers(uris []string) ([]info.Peer, error) {
	peers := []info.Peer{}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	for _, uri := range uris {
		client := info.NewClient(uri)
		nodeID, _, err := client.GetNodeID(ctx)
		if err != nil {
			return nil, err
		}
		ip, err := client.GetNodeIP(ctx)
		if err != nil {
			return nil, err
		}
		peers = append(peers, info.Peer{
			Info: peer.Info{
				ID:       nodeID,
				PublicIP: ip,
			},
		})
	}
	return peers, nil
}

func ConvertToBLSProofOfPossession(publicKey, proofOfPossesion string) (signer.ProofOfPossession, error) {
	type jsonProofOfPossession struct {
		PublicKey         string
		ProofOfPossession string
	}
	jsonPop := jsonProofOfPossession{
		PublicKey:         publicKey,
		ProofOfPossession: proofOfPossesion,
	}
	popBytes, err := json.Marshal(jsonPop)
	if err != nil {
		return signer.ProofOfPossession{}, err
	}
	pop := &signer.ProofOfPossession{}
	err = pop.UnmarshalJSON(popBytes)
	if err != nil {
		return signer.ProofOfPossession{}, err
	}
	return *pop, nil
}

func UpdatePChainHeight(
	title string,
) error {
	_, err := ux.TimedProgressBar(
		30*time.Second,
		title,
		0,
	)
	if err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func GetBlockchainTimestamp(network models.Network) (time.Time, error) {
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	platformCli := platformvm.NewClient(network.Endpoint)
	return platformCli.GetTimestamp(ctx)
}

func GetSubnet(subnetID ids.ID, network models.Network) (platformvm.GetSubnetClientResponse, error) {
	api := network.Endpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return pClient.GetSubnet(ctx, subnetID)
}

func GetSubnetIDFromBlockchainID(blockchainID ids.ID, network models.Network) (ids.ID, error) {
	api := network.Endpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return pClient.ValidatedBy(ctx, blockchainID)
}

func GetLatestCLISupportedAvalancheGoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error) {
	useVersion, err := GetAvailableAvalancheGoVersions(app, rpcVersion, url)
	if err != nil {
		return "", err
	}
	return useVersion[0], nil
}

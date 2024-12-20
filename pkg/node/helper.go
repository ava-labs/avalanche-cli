// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/set"
)

const (
	HealthCheckPoolTime = 60 * time.Second
	HealthCheckTimeout  = 3 * time.Minute
)

type AvalancheGoVersionSettings struct {
	UseCustomAvalanchegoVersion           string
	UseLatestAvalanchegoReleaseVersion    bool
	UseLatestAvalanchegoPreReleaseVersion bool
	UseAvalanchegoVersionFromSubnet       string
}

type ANRSettings struct {
	GenesisPath          string
	UpgradePath          string
	BootstrapIDs         []string
	BootstrapIPs         []string
	StakingTLSKeyPath    string
	StakingCertKeyPath   string
	StakingSignerKeyPath string
}

func AuthorizedAccessFromSettings(app *application.Avalanche) bool {
	return app.Conf.GetConfigBoolValue(constants.ConfigAuthorizeCloudAccessKey)
}

func CheckCluster(app *application.Avalanche, clusterName string) error {
	_, err := GetClusterNodes(app, clusterName)
	return err
}

func GetClusterNodes(app *application.Avalanche, clusterName string) ([]string, error) {
	if exists, err := CheckClusterExists(app, clusterName); err != nil || !exists {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	clusterNodes := clusterConfig.Nodes
	if len(clusterNodes) == 0 && !clusterConfig.Local {
		return nil, fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	return clusterNodes, nil
}

func CheckClusterExists(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return false, err
		}
	}
	_, ok := clustersConfig.Clusters[clusterName]
	return ok, nil
}

func CheckHostsAreRPCCompatible(app *application.Avalanche, hosts []*models.Host, subnetName string) error {
	incompatibleNodes, err := getRPCIncompatibleNodes(app, hosts, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your VM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		switch sc.VM {
		case models.SubnetEvm:
			ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		case models.CustomVM:
			ux.Logger.PrintToUser("To modify your Custom VM binary: avalanche blockchain upgrade vm %s --config", subnetName)
		}
		ux.Logger.PrintToUser("Yoy can use \"avalanche node upgrade\" to upgrade Avalanche Go and/or Subnet-EVM to their latest versions")
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}
	return nil
}

func getRPCIncompatibleNodes(app *application.Avalanche, hosts []*models.Host, subnetName string) ([]string, error) {
	ux.Logger.PrintToUser("Checking compatibility of node(s) avalanche go RPC protocol version with Subnet EVM RPC of blockchain %s ...", subnetName)
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckAvalancheGoVersion(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if _, rpcVersion, err := ParseAvalancheGoOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), rpcVersion, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to get rpc protocol version for node(s) %s", wgResults.GetErrorHostMap())
	}
	incompatibleNodes := []string{}
	for nodeID, rpcVersionI := range wgResults.GetResultMap() {
		rpcVersion := rpcVersionI.(uint32)
		if rpcVersion != uint32(sc.RPCVersion) {
			incompatibleNodes = append(incompatibleNodes, nodeID)
		}
	}
	if len(incompatibleNodes) > 0 {
		ux.Logger.PrintToUser(fmt.Sprintf("Compatible Avalanche Go RPC version is %d", sc.RPCVersion))
	}
	return incompatibleNodes, nil
}

func ParseAvalancheGoOutput(byteValue []byte) (string, uint32, error) {
	reply := map[string]interface{}{}
	if err := json.Unmarshal(byteValue, &reply); err != nil {
		return "", 0, err
	}
	resultMap := reply["result"]
	resultJSON, err := json.Marshal(resultMap)
	if err != nil {
		return "", 0, err
	}

	nodeVersionReply := info.GetNodeVersionReply{}
	if err := json.Unmarshal(resultJSON, &nodeVersionReply); err != nil {
		return "", 0, err
	}
	return nodeVersionReply.VMVersions["platform"], uint32(nodeVersionReply.RPCProtocolVersion), nil
}

func DisconnectHosts(hosts []*models.Host) {
	for _, host := range hosts {
		_ = host.Disconnect()
	}
}

func getPublicEndpoints(
	app *application.Avalanche,
	clusterName string,
	trackers []*models.Host,
) ([]string, error) {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	publicNodes := clusterConfig.APINodes
	if clusterConfig.Network.Kind == models.Devnet {
		publicNodes = clusterConfig.Nodes
	}
	publicTrackers := utils.Filter(trackers, func(tracker *models.Host) bool {
		return utils.Belongs(publicNodes, tracker.GetCloudID())
	})
	endpoints := utils.Map(publicTrackers, func(tracker *models.Host) string {
		return GetAvalancheGoEndpoint(tracker.IP)
	})
	return endpoints, nil
}

func GetAvalancheGoEndpoint(ip string) string {
	return fmt.Sprintf("http://%s:%d", ip, constants.AvalancheGoAPIPort)
}

func GetUnhealthyNodes(hosts []*models.Host) ([]string, error) {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckHealthy(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if isHealthy, err := parseHealthyOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), isHealthy, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to get health status for node(s) %s", wgResults.GetErrorHostMap())
	}
	return utils.Filter(wgResults.GetNodeList(), func(nodeID string) bool {
		return !wgResults.GetResultMap()[nodeID].(bool)
	}), nil
}

func parseHealthyOutput(byteValue []byte) (bool, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return false, err
	}
	isHealthyInterface, ok := result["result"].(map[string]interface{})
	if ok {
		isHealthy, ok := isHealthyInterface["healthy"].(bool)
		if ok {
			return isHealthy, nil
		}
	}
	return false, fmt.Errorf("unable to parse node healthy status")
}

func WaitForHealthyCluster(
	app *application.Avalanche,
	clusterName string,
	timeout time.Duration,
	poolTime time.Duration,
) error {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Waiting for node(s) in cluster %s to be healthy...", clusterName)
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	cluster, ok := clustersConfig.Clusters[clusterName]
	if !ok {
		return fmt.Errorf("cluster %s does not exist", clusterName)
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts := cluster.GetValidatorHosts(allHosts) // exlude api nodes
	defer DisconnectHosts(hosts)
	startTime := time.Now()
	spinSession := ux.NewUserSpinner()
	spinner := spinSession.SpinToUser("Checking if node(s) are healthy...")
	for {
		unhealthyNodes, err := GetUnhealthyNodes(hosts)
		if err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return err
		}
		if len(unhealthyNodes) == 0 {
			ux.SpinComplete(spinner)
			spinSession.Stop()
			ux.Logger.GreenCheckmarkToUser("Nodes healthy after %d seconds", uint32(time.Since(startTime).Seconds()))
			return nil
		}
		if time.Since(startTime) > timeout {
			ux.SpinFailWithError(spinner, "", fmt.Errorf("cluster not healthy after %d seconds", uint32(timeout.Seconds())))
			spinSession.Stop()
			ux.Logger.PrintToUser("")
			ux.Logger.RedXToUser("Unhealthy Nodes")
			for _, failedNode := range unhealthyNodes {
				ux.Logger.PrintToUser("  " + failedNode)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("cluster not healthy after %d seconds", uint32(timeout.Seconds()))
		}
		time.Sleep(poolTime)
	}
}

func GetClusterNameFromList(app *application.Avalanche) (string, error) {
	clusterNames, err := app.ListClusterNames()
	if err != nil {
		return "", err
	}
	if len(clusterNames) == 0 {
		return "", fmt.Errorf("no Avalanche nodes found that can track the blockchain, please create Avalanche nodes first through `avalanche node create`")
	}
	clusterName, err := app.Prompt.CaptureList(
		"Which cluster of Avalanche nodes would you like to use to track the blockchain?",
		clusterNames,
	)
	if err != nil {
		return "", err
	}
	return clusterName, nil
}

// GetAvalancheGoVersion asks users whether they want to install the newest Avalanche Go version
// or if they want to use the newest Avalanche Go Version that is still compatible with Subnet EVM
// version of their choice
func GetAvalancheGoVersion(app *application.Avalanche, avagoVersion AvalancheGoVersionSettings) (string, error) {
	// skip this logic if custom-avalanchego-version flag is set
	if avagoVersion.UseCustomAvalanchegoVersion != "" {
		return avagoVersion.UseCustomAvalanchegoVersion, nil
	}
	latestReleaseVersion, err := app.Downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
		"",
	)
	if err != nil {
		return "", err
	}
	latestPreReleaseVersion, err := app.Downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
		"",
	)
	if err != nil {
		return "", err
	}

	if !avagoVersion.UseLatestAvalanchegoReleaseVersion && !avagoVersion.UseLatestAvalanchegoPreReleaseVersion && avagoVersion.UseCustomAvalanchegoVersion == "" && avagoVersion.UseAvalanchegoVersionFromSubnet == "" {
		avagoVersion, err = promptAvalancheGoVersionChoice(app, latestReleaseVersion, latestPreReleaseVersion)
		if err != nil {
			return "", err
		}
	}

	var version string
	switch {
	case avagoVersion.UseLatestAvalanchegoReleaseVersion:
		version = latestReleaseVersion
	case avagoVersion.UseLatestAvalanchegoPreReleaseVersion:
		version = latestPreReleaseVersion
	case avagoVersion.UseCustomAvalanchegoVersion != "":
		version = avagoVersion.UseCustomAvalanchegoVersion
	case avagoVersion.UseAvalanchegoVersionFromSubnet != "":
		sc, err := app.LoadSidecar(avagoVersion.UseAvalanchegoVersionFromSubnet)
		if err != nil {
			return "", err
		}
		version, err = GetLatestAvagoVersionForRPC(app, sc.RPCVersion, latestPreReleaseVersion)
		if err != nil {
			return "", err
		}
	}
	return version, nil
}

// promptAvalancheGoVersionChoice sets flags for either using the latest Avalanche Go
// version or using the latest Avalanche Go version that is still compatible with the subnet that user
// wants the cloud server to track
func promptAvalancheGoVersionChoice(app *application.Avalanche, latestReleaseVersion string, latestPreReleaseVersion string) (AvalancheGoVersionSettings, error) {
	versionComments := map[string]string{
		"v1.11.0-fuji": " (recommended for fuji durango)",
	}
	latestReleaseVersionOption := "Use latest Avalanche Go Release Version" + versionComments[latestReleaseVersion]
	latestPreReleaseVersionOption := "Use latest Avalanche Go Pre-release Version" + versionComments[latestPreReleaseVersion]
	subnetBasedVersionOption := "Use the deployed Subnet's VM version that the node will be validating"
	customOption := "Custom"

	txt := "What version of Avalanche Go would you like to install in the node?"
	versionOptions := []string{latestReleaseVersionOption, subnetBasedVersionOption, customOption}
	if latestPreReleaseVersion != latestReleaseVersion {
		versionOptions = []string{latestPreReleaseVersionOption, latestReleaseVersionOption, subnetBasedVersionOption, customOption}
	}
	versionOption, err := app.Prompt.CaptureList(txt, versionOptions)
	if err != nil {
		return AvalancheGoVersionSettings{}, err
	}

	switch versionOption {
	case latestReleaseVersionOption:
		return AvalancheGoVersionSettings{UseLatestAvalanchegoReleaseVersion: true}, nil
	case latestPreReleaseVersionOption:
		return AvalancheGoVersionSettings{UseLatestAvalanchegoPreReleaseVersion: true}, nil
	case customOption:
		useCustomAvalanchegoVersion, err := app.Prompt.CaptureVersion("Which version of AvalancheGo would you like to install? (Use format v1.10.13)")
		if err != nil {
			return AvalancheGoVersionSettings{}, err
		}
		return AvalancheGoVersionSettings{UseCustomAvalanchegoVersion: useCustomAvalanchegoVersion}, nil
	default:
		useAvalanchegoVersionFromSubnet := ""
		for {
			useAvalanchegoVersionFromSubnet, err = app.Prompt.CaptureString("Which Subnet would you like to use to choose the avalanche go version?")
			if err != nil {
				return AvalancheGoVersionSettings{}, err
			}
			_, err = subnet.ValidateSubnetNameAndGetChains(app, []string{useAvalanchegoVersionFromSubnet})
			if err == nil {
				break
			}
			ux.Logger.PrintToUser(fmt.Sprintf("no blockchain named as %s found", useAvalanchegoVersionFromSubnet))
		}
		return AvalancheGoVersionSettings{UseAvalanchegoVersionFromSubnet: useAvalanchegoVersionFromSubnet}, nil
	}
}

func GetLatestAvagoVersionForRPC(app *application.Avalanche, configuredRPCVersion int, latestPreReleaseVersion string) (string, error) {
	desiredAvagoVersion, err := vm.GetLatestAvalancheGoByProtocolVersion(
		app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
	if errors.Is(err, vm.ErrNoAvagoVersion) {
		ux.Logger.PrintToUser("No Avalanchego version found for blockchain. Defaulting to latest pre-release version")
		return latestPreReleaseVersion, nil
	}
	if err != nil {
		return "", err
	}
	return desiredAvagoVersion, nil
}

func GetLocalNodeAvalancheGoBinPath() (string, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return "", err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return "", err
	}
	if len(status.ClusterInfo.NodeInfos) == 0 {
		return "", fmt.Errorf("no nodes found")
	} else {
		return status.ClusterInfo.NodeInfos["node1"].ExecPath, nil
	}
}

func GetRunnningLocalNodeClusterName(app *application.Avalanche) (string, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return "", err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return "", err
	}

	pattern := fmt.Sprintf("%s/%s/([^/]+)", app.GetBaseDir(), constants.LocalDir)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(status.ClusterInfo.GetRootDataDir())
	if len(matches) < 2 {
		return "", fmt.Errorf("clusterName not found in input: %s", status.ClusterInfo.GetRootDataDir())
	}
	return matches[1], nil
}

// connect to running ANR and list local node names
func ListLocalNodeNames() ([]string, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return nil, err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}
	localNodeNames := []string{}
	for localNodeName := range status.ClusterInfo.NodeInfos {
		localNodeNames = append(localNodeNames, localNodeName)
	}
	sort.Strings(localNodeNames)
	return localNodeNames, nil
}

func GetNextNodeName() (string, error) {
	currentNodeNames, _ := ListLocalNodeNames()
	if len(currentNodeNames) == 0 {
		return "", fmt.Errorf("no nodes found")
	} else {
		lastNodeName := currentNodeNames[len(currentNodeNames)-1]
		splitStr := strings.Split(lastNodeName, "node")
		if len(splitStr) != 2 {
			return "", fmt.Errorf("invalid node name format: %s", lastNodeName)
		}
		extractedNumber := splitStr[1]
		nodeNumber, err := strconv.Atoi(extractedNumber)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("node%d", nodeNumber+1), nil
	}
}

func AddNodeInfoToSidecar(sc *models.Sidecar, nodeInfo *rpcpb.NodeInfo, network models.Network) error {
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	rpcEndpoints.Add(models.GetRPCEndpoint(nodeInfo.Uri, networkInfo.BlockchainID.String()))
	wsEndpoints.Add(models.GetWSEndpoint(nodeInfo.Uri, networkInfo.BlockchainID.String()))
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[network.Name()] = networkInfo
	return nil
}

func GetNodeInfo(nodeName string) (*rpcpb.NodeInfo, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return nil, err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}
	nodeInfo := status.ClusterInfo.NodeInfos[nodeName]
	return nodeInfo, nil
}

func GetNodeData(endpoint string) (
	string, // nodeID
	string, // public key
	string, // PoP
	error,
) {
	infoClient := info.NewClient(endpoint)
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	nodeID, proofOfPossession, err := infoClient.GetNodeID(ctx)
	if err != nil {
		return "", "", "", err
	}
	return nodeID.String(),
		"0x" + hex.EncodeToString(proofOfPossession.PublicKey[:]),
		"0x" + hex.EncodeToString(proofOfPossession.ProofOfPossession[:]),
		nil
}

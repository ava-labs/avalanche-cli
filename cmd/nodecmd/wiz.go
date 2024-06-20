// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/metrics"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/docker"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

const (
	healthCheckPoolTime   = 60 * time.Second
	healthCheckTimeout    = 3 * time.Minute
	syncCheckPoolTime     = 10 * time.Second
	syncCheckTimeout      = 1 * time.Minute
	validateCheckPoolTime = 10 * time.Second
	validateCheckTimeout  = 1 * time.Minute
)

var (
	forceSubnetCreate                      bool
	subnetGenesisFile                      string
	useEvmSubnet                           bool
	useCustomSubnet                        bool
	evmVersion                             string
	evmChainID                             uint64
	evmToken                               string
	evmDefaults                            bool
	useLatestEvmReleasedVersion            bool
	useLatestEvmPreReleasedVersion         bool
	customVMRepoURL                        string
	customVMBranch                         string
	customVMBuildScript                    string
	nodeConf                               string
	subnetConf                             string
	chainConf                              string
	validators                             []string
	customGrafanaDashboardPath             string
	teleporterReady                        bool
	runRelayer                             bool
	teleporterVersion                      string
	teleporterMessengerContractAddressPath string
	teleporterMessengerDeployerAddressPath string
	teleporterMessengerDeployerTxPath      string
	teleporterRegistryBydecodePath         string
	deployTeleporterMessenger              bool
	deployTeleporterRegistry               bool
)

func newWizCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiz [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Creates a devnet together with a fully validated subnet into it.",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node wiz command creates a devnet and deploys, sync and validate a subnet into it. It creates the subnet if so needed.
`,
		Args:              cobrautils.RangeArgs(1, 2),
		RunE:              wiz,
		PersistentPostRun: handlePostRun,
	}
	cmd.Flags().BoolVar(&useStaticIP, "use-static-ip", true, "attach static Public IP on cloud servers")
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create node/s in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create node/s in GCP cloud")
	cmd.Flags().StringSliceVar(&cmdLineRegion, "region", []string{}, "create node/s in given region(s). Use comma to separate multiple regions")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().IntSliceVar(&numValidatorsNodes, "num-validators", []int{}, "number of nodes to create per region(s). Use comma to separate multiple numbers for each region in the same order as --region flag")
	cmd.Flags().StringVar(&nodeType, "node-type", "", "cloud instance type. Use 'default' to use recommended default instance type")
	cmd.Flags().StringVar(&cmdLineGCPCredentialsPath, "gcp-credentials", "", "use given GCP credentials")
	cmd.Flags().StringVar(&cmdLineGCPProjectName, "gcp-project", "", "use given GCP project")
	cmd.Flags().StringVar(&cmdLineAlternativeKeyPairName, "alternative-key-pair-name", "", "key pair name to use if default one generates conflicts")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "use default weight/start/duration params for subnet validator")

	cmd.Flags().BoolVar(&forceSubnetCreate, "force-subnet-create", false, "overwrite the existing subnet configuration if one exists")
	cmd.Flags().StringVar(&subnetGenesisFile, "subnet-genesis", "", "file path of the subnet genesis")
	cmd.Flags().BoolVar(&teleporterReady, "teleporter", false, "generate a teleporter-ready vm")
	cmd.Flags().BoolVar(&runRelayer, "relayer", false, "run AWM relayer when deploying the vm")
	cmd.Flags().BoolVar(&useEvmSubnet, "evm-subnet", false, "use Subnet-EVM as the subnet virtual machine")
	cmd.Flags().BoolVar(&useCustomSubnet, "custom-subnet", false, "use a custom VM as the subnet virtual machine")
	cmd.Flags().StringVar(&evmVersion, "evm-version", "", "version of Subnet-EVM to use")
	cmd.Flags().Uint64Var(&evmChainID, "evm-chain-id", 0, "chain ID to use with Subnet-EVM")
	cmd.Flags().StringVar(&evmToken, "evm-token", "", "token name to use with Subnet-EVM")
	cmd.Flags().BoolVar(&evmDefaults, "evm-defaults", false, "use default settings for fees/airdrop/precompiles with Subnet-EVM")
	cmd.Flags().BoolVar(&useLatestEvmReleasedVersion, "latest-evm-version", false, "use latest Subnet-EVM released version")
	cmd.Flags().BoolVar(&useLatestEvmPreReleasedVersion, "latest-pre-released-evm-version", false, "use latest Subnet-EVM pre-released version")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch or commit")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	cmd.Flags().StringVar(&customGrafanaDashboardPath, "add-grafana-dashboard", "", "path to additional grafana dashboard json file")
	cmd.Flags().StringVar(&nodeConf, "node-config", "", "path to avalanchego node configuration for subnet")
	cmd.Flags().StringVar(&subnetConf, "subnet-config", "", "path to the subnet configuration for subnet")
	cmd.Flags().StringVar(&chainConf, "chain-config", "", "path to the chain configuration for subnet")
	cmd.Flags().BoolVar(&useSSHAgent, "use-ssh-agent", false, "use ssh agent for ssh")
	cmd.Flags().StringVar(&sshIdentity, "ssh-agent-identity", "", "use given ssh identity(only for ssh agent). If not set, default will be used.")
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&remoteCLIVersion, "remote-cli-version", "", "install given CLI version on remote nodes. defaults to latest CLI release")
	cmd.Flags().StringSliceVar(&validators, "validators", []string{}, "deploy subnet into given comma separated list of validators. defaults to all cluster nodes")
	cmd.Flags().BoolVar(&addMonitoring, enableMonitoringFlag, false, " set up Prometheus monitoring for created nodes. Please note that this option creates a separate monitoring instance and incures additional cost")
	cmd.Flags().IntSliceVar(&numAPINodes, "num-apis", []int{}, "number of API nodes(nodes without stake) to create in the new Devnet")
	cmd.Flags().IntVar(&iops, "aws-volume-iops", constants.AWSGP3DefaultIOPS, "AWS iops (for gp3, io1, and io2 volume types only)")
	cmd.Flags().IntVar(&throughput, "aws-volume-throughput", constants.AWSGP3DefaultThroughput, "AWS throughput in MiB/s (for gp3 volume type only)")
	cmd.Flags().StringVar(&volumeType, "aws-volume-type", "gp3", "AWS volume type")
	cmd.Flags().IntVar(&volumeSize, "aws-volume-size", constants.CloudServerStorageSize, "AWS volume size in GB")
	cmd.Flags().StringVar(&grafanaPkg, "grafana-pkg", "", "use grafana pkg instead of apt repo(by default), for example https://dl.grafana.com/oss/release/grafana_10.4.1_amd64.deb")
	cmd.Flags().StringVar(&teleporterVersion, "teleporter-version", "latest", "teleporter version to deploy")
	cmd.Flags().StringVar(&teleporterMessengerContractAddressPath, "teleporter-messenger-contract-address-path", "", "path to a teleporter messenger contract address file")
	cmd.Flags().StringVar(&teleporterMessengerDeployerAddressPath, "teleporter-messenger-deployer-address-path", "", "path to a teleporter messenger deployer address file")
	cmd.Flags().StringVar(&teleporterMessengerDeployerTxPath, "teleporter-messenger-deployer-tx-path", "", "path to a teleporter messenger deployer tx file")
	cmd.Flags().StringVar(&teleporterRegistryBydecodePath, "teleporter-registry-bytecode-path", "", "path to a teleporter registry bytecode file")
	cmd.Flags().BoolVar(&deployTeleporterMessenger, "deploy-teleporter-messenger", true, "deploy Teleporter Messenger")
	cmd.Flags().BoolVar(&deployTeleporterRegistry, "deploy-teleporter-registry", true, "deploy Teleporter Registry")
	return cmd
}

func wiz(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := ""
	if len(args) > 1 {
		subnetName = args[1]
	}
	clusterAlreadyExists, err := app.ClusterExists(clusterName)
	if err != nil {
		return err
	}
	if clusterAlreadyExists {
		if err := checkClusterIsADevnet(clusterName); err != nil {
			return err
		}
	}
	if clusterAlreadyExists && subnetName == "" {
		return fmt.Errorf("expecting to add subnet to existing cluster but no subnet-name was provided")
	}
	if subnetName != "" && (!app.SidecarExists(subnetName) || forceSubnetCreate) {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Creating the subnet"))
		ux.Logger.PrintToUser("")
		if err := subnetcmd.CallCreate(
			cmd,
			subnetName,
			forceSubnetCreate,
			subnetGenesisFile,
			useEvmSubnet,
			useCustomSubnet,
			evmVersion,
			evmChainID,
			evmToken,
			evmDefaults,
			useLatestEvmReleasedVersion,
			useLatestEvmPreReleasedVersion,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
		); err != nil {
			return err
		}
		if chainConf != "" || subnetConf != "" || nodeConf != "" {
			if err := subnetcmd.CallConfigure(
				cmd,
				subnetName,
				chainConf,
				subnetConf,
				nodeConf,
			); err != nil {
				return err
			}
		}
	}

	if !clusterAlreadyExists {
		globalNetworkFlags.UseDevnet = true
		if len(useCustomAvalanchegoVersion) == 0 && !useLatestAvalanchegoReleaseVersion && !useLatestAvalanchegoPreReleaseVersion {
			useAvalanchegoVersionFromSubnet = subnetName
		}
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Creating the devnet..."))
		ux.Logger.PrintToUser("")
		// wizSubnet is used to get more metrics sent from node create command on whether if vm is custom or subnetEVM
		wizSubnet = subnetName
		if err := createNodes(cmd, []string{clusterName}); err != nil {
			return err
		}
	} else {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Adding subnet into existing devnet %s..."), clusterName)
	}

	// check all validators are found
	if len(validators) != 0 {
		allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}
		clustersConfig, err := app.LoadClustersConfig()
		if err != nil {
			return err
		}
		cluster, ok := clustersConfig.Clusters[clusterName]
		if !ok {
			return fmt.Errorf("cluster %s does not exist", clusterName)
		}
		hosts := cluster.GetValidatorHosts(allHosts) // exlude api nodes
		_, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}

	if err := waitForHealthyCluster(clusterName, healthCheckTimeout, healthCheckPoolTime); err != nil {
		return err
	}

	if subnetName == "" {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Devnet %s has been created!"), clusterName)
		return nil
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Checking subnet compatibility"))
	ux.Logger.PrintToUser("")
	if err := checkRPCCompatibility(clusterName, subnetName); err != nil {
		return err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Creating the blockchain"))
	ux.Logger.PrintToUser("")
	avoidChecks = true
	if err := deploySubnet(cmd, []string{clusterName, subnetName}); err != nil {
		return err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Adding nodes as subnet validators"))
	ux.Logger.PrintToUser("")
	avoidSubnetValidationChecks = true
	if err := validateSubnet(cmd, []string{clusterName, subnetName}); err != nil {
		return err
	}

	network, err := app.GetClusterNetwork(clusterName)
	if err != nil {
		return err
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return ErrNoSubnetID
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Waiting for nodes to be validating the subnet"))
	ux.Logger.PrintToUser("")
	if err := waitForSubnetValidators(network, clusterName, subnetID, validateCheckTimeout, validateCheckPoolTime); err != nil {
		return err
	}

	isEVMGenesis, _, err := subnetcmd.HasSubnetEVMGenesis(subnetName)
	if err != nil {
		return err
	}

	var awmRelayerHost *models.Host
	if sc.TeleporterReady && sc.RunRelayer && isEVMGenesis {
		// get or set AWM Relayer host and configure/stop service
		awmRelayerHost, err = node.GetAWMRelayerHost(app, clusterName)
		if err != nil {
			return err
		}
		if awmRelayerHost == nil {
			awmRelayerHost, err = chooseAWMRelayerHost(clusterName)
			if err != nil {
				return err
			}
			if err := setAWMRelayerHost(awmRelayerHost); err != nil {
				return err
			}
			if err := setAWMRelayerSecurityGroupRule(clusterName, awmRelayerHost); err != nil {
				return err
			}
		} else {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser(logging.Green.Wrap("Stopping AWM Relayer Service"))
			if err := ssh.RunSSHStopAWMRelayerService(awmRelayerHost); err != nil {
				return err
			}
		}
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Setting the nodes as subnet trackers"))
	ux.Logger.PrintToUser("")
	if err := syncSubnet(cmd, []string{clusterName, subnetName}); err != nil {
		return err
	}
	if err := waitForHealthyCluster(clusterName, healthCheckTimeout, healthCheckPoolTime); err != nil {
		return err
	}
	blockchainID := sc.Networks[network.Name()].BlockchainID
	if blockchainID == ids.Empty {
		return ErrNoBlockchainID
	}
	// update logging
	if addMonitoring {
		// set up subnet logs in Loki
		if err = setUpSubnetLogging(clusterName, subnetName); err != nil {
			return err
		}
	}
	if err := waitForClusterSubnetStatus(clusterName, subnetName, blockchainID, status.Validating, validateCheckTimeout, validateCheckPoolTime); err != nil {
		return err
	}

	if b, err := hasTeleporterDeploys(clusterName); err != nil {
		return err
	} else if b {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Updating Proposer VMs"))
		ux.Logger.PrintToUser("")
		if err := updateProposerVMs(network); err != nil {
			// not going to consider fatal, as teleporter messaging will be working fine after a failed first msg
			ux.Logger.PrintToUser(logging.Yellow.Wrap("failure setting proposer: %s"), err)
		}
	}

	if sc.TeleporterReady && sc.RunRelayer && isEVMGenesis {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Setting up teleporter on subnet"))
		ux.Logger.PrintToUser("")
		flags := teleportercmd.DeployFlags{
			SubnetName: subnetName,
			Network: networkoptions.NetworkFlags{
				ClusterName: clusterName,
			},
			DeployMessenger:              deployTeleporterMessenger,
			DeployRegistry:               deployTeleporterRegistry,
			Version:                      teleporterVersion,
			MessengerContractAddressPath: teleporterMessengerContractAddressPath,
			MessengerDeployerAddressPath: teleporterMessengerDeployerAddressPath,
			MessengerDeployerTxPath:      teleporterMessengerDeployerTxPath,
			RegistryBydecodePath:         teleporterRegistryBydecodePath,
		}
		if err := teleportercmd.CallDeploy([]string{}, flags); err != nil {
			return err
		}
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(logging.Green.Wrap("Starting AWM Relayer Service"))
		ux.Logger.PrintToUser("")
		if err := updateAWMRelayerFunds(network, sc, blockchainID); err != nil {
			return err
		}
		if err := updateAWMRelayerHostConfig(awmRelayerHost, subnetName, clusterName); err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("")
	if clusterAlreadyExists {
		ux.Logger.PrintToUser(logging.Green.Wrap("Devnet %s is now validating subnet %s"), clusterName, subnetName)
	} else {
		ux.Logger.PrintToUser(logging.Green.Wrap("Devnet %s is successfully created and is now validating subnet %s!"), clusterName, subnetName)
	}
	ux.Logger.PrintToUser("")

	ux.Logger.PrintToUser(logging.Green.Wrap("Subnet %s RPC URL: %s"), subnetName, network.BlockchainEndpoint(blockchainID.String()))
	ux.Logger.PrintToUser("")

	if addMonitoring {
		if customGrafanaDashboardPath != "" {
			if err = addCustomDashboard(clusterName, subnetName); err != nil {
				return err
			}
		}
		// no need to check for error, as it's ok not to have monitoring host
		monitoringHosts, _ := ansible.GetInventoryFromAnsibleInventoryFile(app.GetMonitoringInventoryDir(clusterName))
		if len(monitoringHosts) > 0 {
			getMonitoringHint(monitoringHosts[0].IP)
		}
	}

	if err := deployClusterYAMLFile(clusterName, subnetName); err != nil {
		return err
	}
	sendNodeWizMetrics(cmd)
	return nil
}

func hasTeleporterDeploys(
	clusterName string,
) (bool, error) {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return false, err
	}
	for _, deployedSubnetName := range clusterConfig.Subnets {
		deployedSubnetIsEVMGenesis, _, err := subnetcmd.HasSubnetEVMGenesis(deployedSubnetName)
		if err != nil {
			return false, err
		}
		deployedSubnetSc, err := app.LoadSidecar(deployedSubnetName)
		if err != nil {
			return false, err
		}
		if deployedSubnetSc.TeleporterReady && deployedSubnetIsEVMGenesis {
			return true, nil
		}
	}
	return false, nil
}

func updateProposerVMs(
	network models.Network,
) error {
	clusterConfig, err := app.GetClusterConfig(network.ClusterName)
	if err != nil {
		return err
	}
	for _, deployedSubnetName := range clusterConfig.Subnets {
		deployedSubnetIsEVMGenesis, _, err := subnetcmd.HasSubnetEVMGenesis(deployedSubnetName)
		if err != nil {
			return err
		}
		deployedSubnetSc, err := app.LoadSidecar(deployedSubnetName)
		if err != nil {
			return err
		}
		if deployedSubnetSc.TeleporterReady && deployedSubnetIsEVMGenesis {
			ux.Logger.PrintToUser("Updating proposerVM on %s", deployedSubnetName)
			blockchainID := deployedSubnetSc.Networks[network.Name()].BlockchainID
			if blockchainID == ids.Empty {
				return ErrNoBlockchainID
			}
			if err := teleporter.SetProposerVM(app, network, blockchainID.String(), deployedSubnetSc.TeleporterKey); err != nil {
				return err
			}
		}
	}
	ux.Logger.PrintToUser("Updating proposerVM on c-chain")
	return teleporter.SetProposerVM(app, network, "C", "")
}

func setAWMRelayerHost(host *models.Host) error {
	cloudID := host.GetCloudID()
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("configuring AWM Relayer on host %s", cloudID)
	nodeConfig, err := app.LoadClusterNodeConfig(cloudID)
	if err != nil {
		return err
	}
	if err := ssh.ComposeSSHSetupAWMRelayer(host); err != nil {
		return err
	}
	nodeConfig.IsAWMRelayer = true
	return app.CreateNodeCloudConfigFile(cloudID, &nodeConfig)
}

func updateAWMRelayerHostConfig(host *models.Host, subnetName string, clusterName string) error {
	ux.Logger.PrintToUser("setting AWM Relayer on host %s to relay subnet %s", host.GetCloudID(), subnetName)
	flags := teleportercmd.AddSubnetToRelayerServiceFlags{
		Network: networkoptions.NetworkFlags{
			ClusterName: clusterName,
		},
		CloudNodeID: host.GetCloudID(),
	}
	if err := teleportercmd.CallAddSubnetToRelayerService(subnetName, flags); err != nil {
		return err
	}
	if err := ssh.RunSSHUploadNodeAWMRelayerConfig(host, app.GetNodeInstanceDirPath(host.GetCloudID())); err != nil {
		return err
	}
	return ssh.RunSSHStartAWMRelayerService(host)
}

func chooseAWMRelayerHost(clusterName string) (*models.Host, error) {
	// first look up for separate monitoring host
	monitoringInventoryFile := app.GetMonitoringInventoryDir(clusterName)
	if utils.FileExists(monitoringInventoryFile) {
		monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryFile)
		if err != nil {
			return nil, err
		}
		if len(monitoringHosts) > 0 {
			return monitoringHosts[0], nil
		}
	}
	// then look up for API nodes
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	if len(clusterConfig.APINodes) > 0 {
		return node.GetHostWithCloudID(app, clusterName, clusterConfig.APINodes[0])
	}
	// finally go for other hosts
	if len(clusterConfig.Nodes) > 0 {
		return node.GetHostWithCloudID(app, clusterName, clusterConfig.Nodes[0])
	}
	return nil, fmt.Errorf("no hosts found on cluster")
}

func updateAWMRelayerFunds(network models.Network, sc models.Sidecar, blockchainID ids.ID) error {
	relayerKey, err := app.GetKey(constants.AWMRelayerKeyName, network, true)
	if err != nil {
		return err
	}
	teleporterKey, err := app.GetKey(sc.TeleporterKey, network, true)
	if err != nil {
		return err
	}
	if err := teleporter.FundRelayer(
		network.BlockchainEndpoint(blockchainID.String()),
		teleporterKey.PrivKeyHex(),
		relayerKey.C(),
	); err != nil {
		return nil
	}
	ewoqKey, err := app.GetKey("ewoq", network, true)
	if err != nil {
		return err
	}
	return teleporter.FundRelayer(
		network.BlockchainEndpoint("C"),
		ewoqKey.PrivKeyHex(),
		relayerKey.C(),
	)
}

func deployClusterYAMLFile(clusterName, subnetName string) error {
	var separateHosts []*models.Host
	var err error
	loadTestInventoryDir := app.GetLoadTestInventoryDir(clusterName)
	if utils.FileExists(loadTestInventoryDir) {
		separateHosts, err = ansible.GetInventoryFromAnsibleInventoryFile(loadTestInventoryDir)
		if err != nil {
			return err
		}
	}
	subnetID, chainID, err := getDeployedSubnetInfo(clusterName, subnetName)
	if err != nil {
		return err
	}
	var externalHost *models.Host
	if len(separateHosts) > 0 {
		externalHost = separateHosts[0]
	}
	if err = createClusterYAMLFile(clusterName, subnetID, chainID, externalHost); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("Cluster information YAML file can be found at %s at local host", app.GetClusterYAMLFilePath(clusterName))
	// deploy YAML file to external host, if it exists
	if len(separateHosts) > 0 {
		if err = ssh.RunSSHCopyYAMLFile(separateHosts[0], app.GetClusterYAMLFilePath(clusterName)); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Cluster information YAML file can be found at /home/ubuntu/%s at external host", constants.ClusterYAMLFileName)
	}
	return nil
}

func checkRPCCompatibility(
	clusterName string,
	subnetName string,
) error {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts := clusterConfig.GetValidatorHosts(allHosts) // exlude api nodes
	if len(validators) != 0 {
		hosts, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}
	defer disconnectHosts(hosts)
	return checkHostsAreRPCCompatible(hosts, subnetName)
}

func waitForHealthyCluster(
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
	defer disconnectHosts(hosts)
	startTime := time.Now()
	spinSession := ux.NewUserSpinner()
	spinner := spinSession.SpinToUser("Checking if node(s) are healthy...")
	for {
		unhealthyNodes, err := getUnhealthyNodes(hosts)
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

func waitForSubnetValidators(
	network models.Network,
	clusterName string,
	subnetID ids.ID,
	timeout time.Duration,
	poolTime time.Duration,
) error {
	ux.Logger.PrintToUser("Waiting for node(s) in cluster %s to be validators of subnet ID %s...", clusterName, subnetID)
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts := clusterConfig.GetValidatorHosts(allHosts) // exlude api nodes
	if len(validators) != 0 {
		hosts, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}
	defer disconnectHosts(hosts)
	nodeIDMap, failedNodesMap := getNodeIDs(hosts)
	startTime := time.Now()
	for {
		failedNodes := []string{}
		for _, host := range hosts {
			nodeID, b := nodeIDMap[host.NodeID]
			if !b {
				err, b := failedNodesMap[host.NodeID]
				if !b {
					return fmt.Errorf("expected to found an error for non mapped node")
				}
				return err
			}
			isValidator, err := subnet.IsSubnetValidator(subnetID, nodeID, network)
			if err != nil {
				return err
			}
			if !isValidator {
				failedNodes = append(failedNodes, host.GetCloudID())
			}
		}
		if len(failedNodes) == 0 {
			ux.Logger.PrintToUser("Nodes validating subnet ID %s after %d seconds", subnetID, uint32(time.Since(startTime).Seconds()))
			return nil
		}
		if time.Since(startTime) > timeout {
			ux.Logger.PrintToUser("Nodes not validating subnet ID %sf", subnetID)
			for _, failedNode := range failedNodes {
				ux.Logger.PrintToUser("  " + failedNode)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("cluster %s not validating subnet ID %s after %d seconds", clusterName, subnetID, uint32(timeout.Seconds()))
		}
		time.Sleep(poolTime)
	}
}

func waitForClusterSubnetStatus(
	clusterName string,
	subnetName string,
	blockchainID ids.ID,
	targetStatus status.BlockchainStatus,
	timeout time.Duration,
	poolTime time.Duration,
) error {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Waiting for node(s) in cluster %s to be %s subnet %s...", clusterName, strings.ToLower(targetStatus.String()), subnetName)
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
	if len(validators) != 0 {
		hosts, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}
	defer disconnectHosts(hosts)
	startTime := time.Now()
	for {
		wg := sync.WaitGroup{}
		wgResults := models.NodeResults{}
		for _, host := range hosts {
			wg.Add(1)
			go func(nodeResults *models.NodeResults, host *models.Host) {
				defer wg.Done()
				if syncstatus, err := ssh.RunSSHSubnetSyncStatus(host, blockchainID.String()); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				} else {
					if subnetSyncStatus, err := parseSubnetSyncOutput(syncstatus); err != nil {
						nodeResults.AddResult(host.NodeID, nil, err)
						return
					} else {
						nodeResults.AddResult(host.NodeID, subnetSyncStatus, err)
					}
				}
			}(&wgResults, host)
		}
		wg.Wait()
		if wgResults.HasErrors() {
			return fmt.Errorf("failed to check sync status for node(s) %s", wgResults.GetErrorHostMap())
		}
		failedNodes := []string{}
		for host, subnetSyncStatus := range wgResults.GetResultMap() {
			if subnetSyncStatus != targetStatus.String() {
				failedNodes = append(failedNodes, host)
			}
		}
		if len(failedNodes) == 0 {
			ux.Logger.PrintToUser("Nodes %s %s after %d seconds", targetStatus.String(), subnetName, uint32(time.Since(startTime).Seconds()))
			return nil
		}
		if time.Since(startTime) > timeout {
			ux.Logger.PrintToUser("Nodes not %s %s", targetStatus.String(), subnetName)
			for _, failedNode := range failedNodes {
				ux.Logger.PrintToUser("  " + failedNode)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("cluster not %s subnet %s after %d seconds", strings.ToLower(targetStatus.String()), subnetName, uint32(timeout.Seconds()))
		}
		time.Sleep(poolTime)
	}
}

func checkClusterIsADevnet(clusterName string) error {
	exists, err := app.ClusterExists(clusterName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("cluster %q does not exists", clusterName)
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	if clustersConfig.Clusters[clusterName].Network.Kind != models.Devnet {
		return fmt.Errorf("cluster %q is not a Devnet", clusterName)
	}
	return nil
}

func filterHosts(hosts []*models.Host, nodes []string) ([]*models.Host, error) {
	indices := set.Set[int]{}
	for _, node := range nodes {
		added := false
		for i, host := range hosts {
			cloudID := host.GetCloudID()
			ip := host.IP
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudID))
			if err != nil {
				return nil, err
			}
			if slices.Contains([]string{cloudID, ip, nodeID.String()}, node) {
				added = true
				indices.Add(i)
			}
		}
		if !added {
			return nil, fmt.Errorf("node %q not found", node)
		}
	}
	filteredHosts := []*models.Host{}
	for i, host := range hosts {
		if indices.Contains(i) {
			filteredHosts = append(filteredHosts, host)
		}
	}
	return filteredHosts, nil
}

func setAWMRelayerSecurityGroupRule(clusterName string, awmRelayerHost *models.Host) error {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	hasGCPNodes := false
	lastRegion := ""
	var ec2Svc *awsAPI.AwsCloud
	for _, cloudID := range clusterConfig.GetCloudIDs() {
		nodeConfig, err := app.LoadClusterNodeConfig(cloudID)
		if err != nil {
			return err
		}
		switch {
		case nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService:
			if nodeConfig.Region != lastRegion {
				ec2Svc, err = awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
				if err != nil {
					return err
				}
				lastRegion = nodeConfig.Region
			}
			securityGroupExists, sg, err := ec2Svc.CheckSecurityGroupExists(nodeConfig.SecurityGroup)
			if err != nil {
				return err
			}
			if !securityGroupExists {
				return fmt.Errorf("security group %s doesn't exist in region %s", nodeConfig.SecurityGroup, nodeConfig.Region)
			}
			if inSG := awsAPI.CheckIPInSg(&sg, awmRelayerHost.IP, constants.AvalanchegoAPIPort); !inSG {
				if err = ec2Svc.AddSecurityGroupRule(
					*sg.GroupId,
					"ingress",
					"tcp",
					awmRelayerHost.IP+constants.IPAddressSuffix,
					constants.AvalanchegoAPIPort,
				); err != nil {
					return err
				}
			}
		case nodeConfig.CloudService == constants.GCPCloudService:
			hasGCPNodes = true
		default:
			return fmt.Errorf("cloud %s is not supported", nodeConfig.CloudService)
		}
	}
	if hasGCPNodes {
		if err := setGCPAWMRelayerSecurityGroupRule(awmRelayerHost); err != nil {
			return err
		}
	}
	return nil
}

func sendNodeWizMetrics(cmd *cobra.Command) {
	flags := make(map[string]string)
	populateSubnetVMMetrics(flags, wizSubnet)
	metrics.HandleTracking(cmd, constants.MetricsNodeDevnetWizCommand, app, flags)
}

func populateSubnetVMMetrics(flags map[string]string, subnetName string) {
	sc, err := app.LoadSidecar(subnetName)
	if err == nil {
		switch sc.VM {
		case models.SubnetEvm:
			flags[constants.MetricsSubnetVM] = "Subnet-EVM"
		case models.CustomVM:
			flags[constants.MetricsSubnetVM] = "Custom-VM"
			flags[constants.MetricsCustomVMRepoURL] = sc.CustomVMRepoURL
			flags[constants.MetricsCustomVMBranch] = sc.CustomVMBranch
			flags[constants.MetricsCustomVMBuildScript] = sc.CustomVMBuildScript
		}
	}
	flags[constants.MetricsEnableMonitoring] = strconv.FormatBool(addMonitoring)
}

// setUPSubnetLogging sets up the subnet logging for the subnet
func setUpSubnetLogging(clusterName, subnetName string) error {
	_, chainID, err := getDeployedSubnetInfo(clusterName, subnetName)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	spinSession := ux.NewUserSpinner()
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
	monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if !addMonitoring {
			continue
		}
		wg.Add(1)
		go func(host *models.Host) {
			defer wg.Done()
			spinner := spinSession.SpinToUser(utils.ScriptLog(host.NodeID, "Setup Subnet Logs"))
			cloudID := host.GetCloudID()
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudID))
			if err != nil {
				wgResults.AddResult(host.NodeID, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			if err = ssh.RunSSHSetupPromtailConfig(host, monitoringHosts[0].IP, constants.AvalanchegoLokiPort, cloudID, nodeID.String(), chainID); err != nil {
				wgResults.AddResult(host.NodeID, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			if err := docker.RestartDockerComposeService(host, utils.GetRemoteComposeFile(), "promtail", constants.SSHLongRunningScriptTimeout); err != nil {
				wgResults.AddResult(host.NodeID, nil, err)
				ux.SpinFailWithError(spinner, "", err)
				return
			}
			ux.SpinComplete(spinner)
		}(host)
	}
	wg.Wait()
	for _, node := range hosts {
		if wgResults.HasNodeIDWithError(node.NodeID) {
			ux.Logger.RedXToUser("Node %s is ERROR with error: %s", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
		}
	}
	spinSession.Stop()
	return nil
}

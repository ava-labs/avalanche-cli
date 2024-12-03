// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	"time"
)

type HTTPAccess bool

const (
	DefaultPerms755        = 0o755
	WriteReadReadPerms     = 0o644
	WriteReadUserOnlyPerms = 0o600

	UbuntuVersionLTS = "20.04"

	BaseDirName = ".avalanche-cli"
	LogDir      = "logs"

	ServerRunFile                   = "gRPCserver.run"
	ServerRunFileLocalNetworkPrefix = ""
	ServerRunFileLocalClusterPrefix = "localcluster_"

	AvalancheCliBinDir = "bin"
	RunDir             = "runs"
	ServicesDir        = "services"

	SuffixSeparator              = "_"
	SidecarFileName              = "sidecar.json"
	GenesisFileName              = "genesis.json"
	UpgradeFileName              = "upgrade.json"
	AliasesFileName              = "aliases.json"
	SidecarSuffix                = SuffixSeparator + SidecarFileName
	GenesisSuffix                = SuffixSeparator + GenesisFileName
	NodeFileName                 = "node.json"
	NodePrometheusConfigFileName = "prometheus.yml"
	NodeCloudConfigFileName      = "node_cloud_config.json"
	AnsibleDir                   = "ansible"
	AnsibleHostInventoryFileName = "hosts"
	StopAWSNode                  = "stop-aws-node"
	CreateAWSNode                = "create-aws-node"
	GetAWSNodeIP                 = "get-aws-node-ip"
	ClustersConfigFileName       = "cluster_config.json"
	ClustersConfigVersion        = "1"
	StakerCertFileName           = "staker.crt"
	StakerKeyFileName            = "staker.key"
	BLSKeyFileName               = "signer.key"
	SidecarVersion               = "1.4.0"

	MaxLogFileSize   = 4
	MaxNumOfLogFiles = 5
	RetainOldFiles   = 0 // retain all old log files

	CloudOperationTimeout = 2 * time.Minute

	ANRRequestTimeout      = 3 * time.Minute
	APIRequestTimeout      = 10 * time.Second
	APIRequestLargeTimeout = 10 * time.Second
	FastGRPCDialTimeout    = 100 * time.Millisecond

	FujiBootstrapTimeout = 15 * time.Minute

	SSHServerStartTimeout       = 1 * time.Minute
	SSHScriptTimeout            = 2 * time.Minute
	SSHLongRunningScriptTimeout = 10 * time.Minute
	SSHDirOpsTimeout            = 10 * time.Second
	SSHFileOpsTimeout           = 100 * time.Second
	SSHPOSTTimeout              = 10 * time.Second
	SSHSleepBetweenChecks       = 1 * time.Second
	SSHShell                    = "/bin/bash"
	AWSVolumeTypeGP3            = "gp3"
	AWSVolumeTypeIO1            = "io1"
	AWSVolumeTypeIO2            = "io2"
	AWSGP3DefaultIOPS           = 3000
	AWSGP3DefaultThroughput     = 125
	SimulatePublicNetwork       = "SIMULATE_PUBLIC_NETWORK"
	OperateOfflineEnvVarName    = "CLIOFFLINE"

	LatestPreReleaseVersionTag = "latest-prerelease"
	LatestReleaseVersionTag    = "latest"
	DefaultAvalancheGoVersion  = LatestPreReleaseVersionTag

	// TODO: remove after etna release is available
	FirstEtnaRPCVersion = 38

	PublicAccess       HTTPAccess = true
	PrivateAccess      HTTPAccess = false
	FujiAPIEndpoint               = "https://api.avax-test.network"
	MainnetAPIEndpoint            = "https://api.avax.network"

	// this depends on bootstrap snapshot
	LocalAPIEndpoint                   = "http://127.0.0.1:9650"
	LocalNetworkID                     = 1337
	LocalNetworkNumNodes               = 2
	LocalNetworkAvalancheGoMaxLogSize  = 1
	LocalNetworkAvalancheGoMaxLogFiles = 2

	DevnetAPIEndpoint = ""
	DevnetNetworkID   = 1338

	DefaultTokenName = "Test Token"

	DefaultTokenSymbol = "TEST"

	HealthCheckInterval = 100 * time.Millisecond

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	SnapshotsDirName = "snapshots"

	DefaultSnapshotName = "default-1654102509"

	Cortina17Version = "v1.10.17"
	Durango11Version = "v1.11.11"
	Durango12Version = "v1.11.12"

	BootstrapSnapshotRawBranch = "https://github.com/ava-labs/avalanche-cli/raw/main/"

	ExtraLocalNetworkDataFilename = "extra-local-network-data.json"

	CliInstallationURL         = "https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh"
	ExpectedCliInstallErr      = "resource temporarily unavailable"
	EIPLimitErr                = "AddressLimitExceeded"
	ErrCreatingAWSNode         = "failed to create AWS Node"
	ErrCreatingGCPNode         = "failed to create GCP Node"
	ErrReleasingGCPStaticIP    = "failed to release gcp static ip"
	KeyDir                     = "key"
	KeySuffix                  = ".pk"
	YAMLSuffix                 = ".yml"
	CustomGrafanaDashboardJSON = "custom.json"
	Enable                     = "enable"

	Disable = "disable"

	TimeParseLayout = "2006-01-02 15:04:05"
	MinStakeWeight  = 1
	// Default balance when we prompt users for bootstrap validators
	// nAVAX
	BootstrapValidatorBalance = 1000000000
	// Default weight when we prompt users for bootstrap validators
	BootstrapValidatorWeight = 100
	// Default weight when we prompt users for non bootstrap validators
	NonBootstrapValidatorWeight       = BootstrapValidatorWeight / 5
	DefaultStakeWeight                = 20
	AVAXSymbol                        = "AVAX"
	DefaultFujiStakeDuration          = "48h"
	DefaultMainnetStakeDuration       = "336h"
	DefaultValidationIDExpiryDuration = 24 * time.Hour
	// The absolute minimum is 25 seconds, but set to 1 minute to allow for
	// time to go through the command
	DevnetStakingStartLeadTime                   = 30 * time.Second
	StakingStartLeadTime                         = 5 * time.Minute
	StakingMinimumLeadTime                       = 25 * time.Second
	PrimaryNetworkValidatingStartLeadTimeNodeCmd = 20 * time.Second
	PrimaryNetworkValidatingStartLeadTime        = 1 * time.Minute
	AWSCloudServerRunningState                   = "running"
	AvalancheCLISuffix                           = "-avalanche-cli"
	AWSDefaultCredential                         = "default"
	GCPDefaultImageProvider                      = "avalabs-experimental"
	GCPImageFilter                               = "family=avalanchecli-ubuntu-2204 AND architecture=x86_64"
	GCPEnvVar                                    = "GOOGLE_APPLICATION_CREDENTIALS"
	GCPDefaultAuthKeyPath                        = "~/.config/gcloud/application_default_credentials.json"
	CertSuffix                                   = "-kp.pem"
	AWSSecurityGroupSuffix                       = "-sg"
	ExportSubnetSuffix                           = "-export.dat"
	SSHTCPPort                                   = 22
	AvalancheGoAPIPort                           = 9650
	AvalancheGoP2PPort                           = 9651
	AvalancheGoGrafanaPort                       = 3000
	AvalancheGoLokiPort                          = 23101
	CloudServerStorageSize                       = 1000
	MonitoringCloudServerStorageSize             = 50
	OutboundPort                                 = 0
	// Set this one to true while testing changes that alter CLI execution on cloud nodes
	// Disable it for releases to save cluster creation time
	EnableSetupCLIFromSource           = false
	SetupCLIFromSourceBranch           = "main"
	BuildEnvGolangVersion              = "1.22.1"
	IsHealthyJSONFile                  = "isHealthy.json"
	IsBootstrappedJSONFile             = "isBootstrapped.json"
	AvalancheGoVersionJSONFile         = "avalancheGoVersion.json"
	SubnetSyncJSONFile                 = "isSubnetSynced.json"
	AnsibleInventoryDir                = "inventories"
	AnsibleTempInventoryDir            = "temp_inventories"
	AnsibleStatusDir                   = "status"
	AnsibleInventoryFlag               = "-i"
	AnsibleExtraArgsIdentitiesOnlyFlag = "--ssh-extra-args='-o IdentitiesOnly=yes'"
	AnsibleSSHShellParams              = "-o IdentitiesOnly=yes -o StrictHostKeyChecking=no"
	AnsibleSSHUseAgentParams           = "-o StrictHostKeyChecking=no"
	AnsibleExtraVarsFlag               = "--extra-vars"

	ConfigAPMCredentialsFileKey   = "credentials-file"
	ConfigAPMAdminAPIEndpointKey  = "admin-api-endpoint"
	ConfigNodeConfigKey           = "node-config"
	ConfigMetricsEnabledKey       = "MetricsEnabled"
	ConfigAuthorizeCloudAccessKey = "AuthorizeCloudAccess"
	ConfigSnapshotsAutoSaveKey    = "SnapshotsAutoSaveEnabled"
	OldConfigFileName             = ".avalanche-cli.json"
	OldMetricsConfigFileName      = ".avalanche-cli/config"
	DefaultConfigFileName         = ".avalanche-cli/config.json"
	DefaultNodeType               = "default"
	AWSCloudService               = "Amazon Web Services"
	GCPCloudService               = "Google Cloud Platform"
	AWSDefaultInstanceType        = "c5.2xlarge"
	GCPDefaultInstanceType        = "e2-standard-8"
	AnsibleSSHUser                = "ubuntu"
	AWSNodeAnsiblePrefix          = "aws_node"
	GCPNodeAnsiblePrefix          = "gcp_node"
	CustomVMDir                   = "vms"
	ClusterYAMLFileName           = "clusterInfo.yaml"
	GCPStaticIPPrefix             = "static-ip"
	AvaLabsOrg                    = "ava-labs"
	AvalancheGoRepoName           = "avalanchego"
	SubnetEVMRepoName             = "subnet-evm"
	CliRepoName                   = "avalanche-cli"
	TeleporterRepoName            = "teleporter"
	ICMServicesRepoName           = "icm-services"
	ICMRelayerKind                = "icm-relayer"
	SubnetEVMReleaseURL           = "https://github.com/ava-labs/subnet-evm/releases/download/%s/%s"
	SubnetEVMArchive              = "subnet-evm_%s_linux_amd64.tar.gz"
	CloudNodeConfigBasePath       = "/home/ubuntu/.avalanchego/"
	CloudNodeSubnetEvmBinaryPath  = "/home/ubuntu/.avalanchego/plugins/%s"
	CloudNodeStakingPath          = "/home/ubuntu/.avalanchego/staking/"
	CloudNodeConfigPath           = "/home/ubuntu/.avalanchego/configs/"
	CloudNodePluginsPath          = "/home/ubuntu/.avalanchego/plugins/"
	DockerNodeConfigPath          = "/.avalanchego/configs/"
	CloudNodePrometheusConfigPath = "/etc/prometheus/prometheus.yml"
	CloudNodeCLIConfigBasePath    = "/home/ubuntu/.avalanche-cli/"
	AvalancheGoMonitoringPort     = 9090
	AvalancheGoMachineMetricsPort = 9100
	MonitoringDir                 = "monitoring"
	LoadTestDir                   = "loadtest"
	DashboardsDir                 = "dashboards"
	NodeConfigJSONFile            = "node.json"
	IPAddressSuffix               = "/32"
	AvalancheGoInstallDir         = "avalanchego"
	SubnetEVMInstallDir           = "subnet-evm"
	ICMRelayerInstallDir          = "icm-relayer"
	TeleporterInstallDir          = "teleporter"
	ICMRelayerBin                 = "icm-relayer"
	LocalRelayerDir               = "local-relayer"
	ICMRelayerConfigFilename      = "icm-relayer-config.json"
	ICMRelayerStorageDir          = "icm-relayer-storage"
	ICMRelayerLogFilename         = "icm-relayer.log"
	ICMRelayerRunFilename         = "icm-relayer-process.json"
	ICMRelayerDockerDir           = "/.icm-relayer"

	ICMRelayerSnapshotConfsDir = "relayer-confs"

	ICMKeyName           = "cli-teleporter-deployer"
	ICMRelayerKeyName    = "cli-awm-relayer"
	DefaultRelayerAmount = float64(10)

	// to not interfere with other node services
	RemoteICMRelayerMetricsPort = 9091

	// enables having many local relayers
	LocalNetworkLocalICMRelayerMetricsPort = 9092
	DevnetLocalICMRelayerMetricsPort       = 9093
	EtnaDevnetLocalICMRelayerMetricsPort   = 9094
	FujiLocalICMRelayerMetricsPort         = 9095

	DevnetFlagsProposerVMUseCurrentHeight = true

	SubnetEVMBin = "subnet-evm"

	DefaultNodeRunURL = "http://127.0.0.1:9650"

	APMDir                = ".apm"
	APMLogName            = "apm.log"
	DefaultAvaLabsPackage = "ava-labs/avalanche-plugins-core"
	APMPluginDir          = "apm_plugins"

	// #nosec G101
	GithubAPITokenEnvVarName = "AVALANCHE_CLI_GITHUB_TOKEN"

	ReposDir                    = "repos"
	SubnetDir                   = "subnets"
	NodesDir                    = "nodes"
	VMDir                       = "vms"
	ChainConfigDir              = "chains"
	AVMKeyName                  = "avm"
	EVMKeyName                  = "evm"
	PlatformKeyName             = "platform"
	MetricsCloudService         = "cloud-service"
	MetricsNodeType             = "node-type"
	MetricsAWSVolumeType        = "aws-volume-type"
	MetricsAWSVolumeSize        = "aws-volume-size"
	MetricsUseStaticIP          = "use-static-ip"
	MetricsValidatorCount       = "num-validator-nodes"
	MetricsAPICount             = "num-api-nodes"
	MetricsEnableMonitoring     = "enable-monitoring"
	MetricsSubnetVM             = "subnet-vm"
	MetricsCustomVMRepoURL      = "custom-vm-repo-url"
	MetricsCustomVMBranch       = "custom-vm-branch"
	MetricsCustomVMBuildScript  = "custom-vm-build-script"
	MetricsCalledFromWiz        = "called-from-wiz"
	MetricsNumRegions           = "num-region"
	MetricsNodeCreateCommand    = "avalanche node create"
	MetricsNodeDevnetWizCommand = "avalanche node devnet wiz"
	MetricsSubnetDeployCommand  = "avalanche subnet deploy"
	MetricsSubnetCreateCommand  = "avalanche subnet create"
	SubnetType                  = "subnet type"
	PrecompileType              = "precompile type"
	CustomAirdrop               = "custom-airdrop"
	NumberOfAirdrops            = "airdrop-addresses"
	SubnetConfigFileName        = "subnet.json"
	ChainConfigFileName         = "chain.json"
	PerNodeChainConfigFileName  = "per-node-chain.json"
	NodeConfigFileName          = "node-config.json"

	GitRepoCommitName  = "Avalanche-CLI"
	GitRepoCommitEmail = "info@avax.network"
	AvaLabsMaintainers = "ava-labs"

	AvalancheGoDockerImage = "avaplatform/avalanchego"
	AvalancheGoGitRepo     = "https://github.com/ava-labs/avalanchego"

	UpgradeBytesLockExtension = ".lock"
	NotAvailableLabel         = "Not available"
	BackendCmd                = "avalanche-cli-backend"

	AvalancheGoVersionUnknown            = "n/a"
	AvalancheGoCompatibilityVersionAdded = "v1.9.2"
	AvalancheGoCompatibilityURL          = "https://raw.githubusercontent.com/ava-labs/avalanchego/master/version/compatibility.json"
	SubnetEVMRPCCompatibilityURL         = "https://raw.githubusercontent.com/ava-labs/subnet-evm/master/compatibility.json"

	YesLabel = "Yes"
	NoLabel  = "No"

	SubnetIDLabel     = "SubnetID: "
	BlockchainIDLabel = "BlockchainID: "

	PluginDir = "plugins"
	LocalDir  = "local"

	DefaultNumberOfLocalMachineNodes = 1
	MetricsNetwork                   = "network"
	MultiSig                         = "multi-sig"
	SkipUpdateFlag                   = "skip-update-check"
	LastFileName                     = ".last_actions.json"
	APIRole                          = "API"
	ValidatorRole                    = "Validator"
	MonitorRole                      = "Monitor"
	ICMRelayerRole                   = "Relayer"
	LoadTestRole                     = "LoadTest"
	DefaultWalletCreationTimeout     = 5 * time.Second

	DefaultConfirmTxTimeout = 20 * time.Second

	PayTxsFeesMsg = "pay transaction fees"

	CodespaceNameEnvVar = "CODESPACE_NAME"

	// E2E
	E2ENetworkPrefix        = "192.168.222"
	E2EListenPrefix         = "192.168.223"
	E2EClusterName          = "e2e"
	E2EDocker               = "docker"
	E2EDockerComposeFile    = "/tmp/avalanche-cli-docker-compose.yml"
	E2EDebugAvalancheGoPath = "E2E_AVALANCHEGO_PATH"
	GitExtension            = ".git"

	// Docker
	RemoteDockeSocketPath = "/var/run/docker.sock"

	// Avalanche InterChain Token Transfer
	ICTTDir     = "avalanche-interchain-token-transfer"
	ICTTURL     = "https://github.com/ava-labs/avalanche-interchain-token-transfer"
	ICTTBranch  = "main"
	ICTTVersion = "v1.0.0"

	// ICM
	DefaultTeleporterMessengerAddress         = "0x253b2784c75e510dD0fF1da844684a1aC0aa5fcf"
	MainnetCChainTeleporterRegistryAddress    = "0x7C43605E14F391720e1b37E49C78C4b03A488d98"
	FujiCChainTeleporterRegistryAddress       = "0xF86Cb19Ad8405AEFa7d09C778215D2Cb6eBfB228"
	EtnaDevnetCChainTeleporterRegistryAddress = "0xEe40DFF876204A99eCCB783FDc01eE0a2678Ae93"
)

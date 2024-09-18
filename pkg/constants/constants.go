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

	ServerRunFile      = "gRPCserver.run"
	AvalancheCliBinDir = "bin"
	RunDir             = "runs"
	ServicesDir        = "services"

	SuffixSeparator              = "_"
	SidecarFileName              = "sidecar.json"
	GenesisFileName              = "genesis.json"
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
	APIRequestTimeout      = 30 * time.Second
	APIRequestLargeTimeout = 2 * time.Minute
	FastGRPCDialTimeout    = 100 * time.Millisecond

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

	OperateOfflineEnvVarName = "CLIOFFLINE"

	PublicAccess       HTTPAccess = true
	PrivateAccess      HTTPAccess = false
	FujiAPIEndpoint               = "https://api.avax-test.network"
	MainnetAPIEndpoint            = "https://api.avax.network"

	// this depends on bootstrap snapshot
	LocalAPIEndpoint = "http://127.0.0.1:9650"
	LocalNetworkID   = 1337

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

	BootstrapSnapshotRawBranch = "https://github.com/ava-labs/avalanche-cli/raw/main/"

	CurrentBootstrapNamePath = "currentBootstrapName.txt"

	AssetsDir = "assets/"

	BootstrapSnapshotArchiveName = "bootstrapSnapshot.tar.gz"
	BootstrapSnapshotLocalPath   = AssetsDir + BootstrapSnapshotArchiveName
	BootstrapSnapshotURL         = BootstrapSnapshotRawBranch + BootstrapSnapshotLocalPath
	BootstrapSnapshotSHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sum.txt"

	BootstrapSnapshotSingleNodeArchiveName = "bootstrapSnapshotSingleNode.tar.gz"
	BootstrapSnapshotSingleNodeLocalPath   = AssetsDir + BootstrapSnapshotSingleNodeArchiveName
	BootstrapSnapshotSingleNodeURL         = BootstrapSnapshotRawBranch + BootstrapSnapshotSingleNodeLocalPath
	BootstrapSnapshotSingleNodeSHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sumSingleNode.txt"

	BootstrapSnapshotPreCortina17ArchiveName = "bootstrapSnapshot.PreCortina17.tar.gz"
	BootstrapSnapshotPreCortina17LocalPath   = AssetsDir + BootstrapSnapshotPreCortina17ArchiveName
	BootstrapSnapshotPreCortina17URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotPreCortina17LocalPath
	BootstrapSnapshotPreCortina17SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sum.PreCortina17.txt"

	BootstrapSnapshotSingleNodePreCortina17ArchiveName = "bootstrapSnapshotSingleNode.PreCortina17.tar.gz"
	BootstrapSnapshotSingleNodePreCortina17LocalPath   = AssetsDir + BootstrapSnapshotSingleNodePreCortina17ArchiveName
	BootstrapSnapshotSingleNodePreCortina17URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotSingleNodePreCortina17LocalPath
	BootstrapSnapshotSingleNodePreCortina17SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sumSingleNode.PreCortina17.txt"

	BootstrapSnapshotPreDurango11ArchiveName = "bootstrapSnapshot.PreDurango11.tar.gz"
	BootstrapSnapshotPreDurango11LocalPath   = AssetsDir + BootstrapSnapshotPreDurango11ArchiveName
	BootstrapSnapshotPreDurango11URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotPreDurango11LocalPath
	BootstrapSnapshotPreDurango11SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sum.PreDurango11.txt"

	BootstrapSnapshotSingleNodePreDurango11ArchiveName = "bootstrapSnapshotSingleNode.PreDurango11.tar.gz"
	BootstrapSnapshotSingleNodePreDurango11LocalPath   = AssetsDir + BootstrapSnapshotSingleNodePreDurango11ArchiveName
	BootstrapSnapshotSingleNodePreDurango11URL         = BootstrapSnapshotRawBranch + BootstrapSnapshotSingleNodePreDurango11LocalPath
	BootstrapSnapshotSingleNodePreDurango11SHA256URL   = BootstrapSnapshotRawBranch + AssetsDir + "sha256sumSingleNode.PreDurango11.txt"

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

	TimeParseLayout             = "2006-01-02 15:04:05"
	MinStakeWeight              = 1
	DefaultStakeWeight          = 20
	AVAXSymbol                  = "AVAX"
	DefaultFujiStakeDuration    = "48h"
	DefaultMainnetStakeDuration = "336h"
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
	AvalanchegoAPIPort                           = 9650
	AvalanchegoP2PPort                           = 9651
	AvalanchegoGrafanaPort                       = 3000
	AvalanchegoLokiPort                          = 23101
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
	ConfigSingleNodeEnabledKey    = "SingleNodeEnabled"
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
	AWMRelayerRepoName            = "awm-relayer"
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
	AvalanchegoMonitoringPort     = 9090
	AvalanchegoMachineMetricsPort = 9100
	MonitoringDir                 = "monitoring"
	LoadTestDir                   = "loadtest"
	DashboardsDir                 = "dashboards"
	NodeConfigJSONFile            = "node.json"
	IPAddressSuffix               = "/32"
	AvalancheGoInstallDir         = "avalanchego"
	SubnetEVMInstallDir           = "subnet-evm"
	AWMRelayerInstallDir          = "awm-relayer"
	TeleporterInstallDir          = "teleporter"
	AWMRelayerBin                 = "awm-relayer"
	LocalRelayerDir               = "local-relayer"
	AWMRelayerConfigFilename      = "awm-relayer-config.json"
	AWMRelayerStorageDir          = "awm-relayer-storage"
	AWMRelayerLogFilename         = "awm-relayer.log"
	AWMRelayerRunFilename         = "awm-relayer-process.json"
	AWMRelayerDockerDir           = "/.awm-relayer"

	AWMRelayerSnapshotConfsDir = "relayer-confs"

	ICMKeyName        = "cli-teleporter-deployer"
	AWMRelayerKeyName = "cli-awm-relayer"

	// to not interfere with other node services
	RemoteAWMRelayerMetricsPort = 9091

	// enables having many local relayers
	LocalNetworkLocalAWMRelayerMetricsPort = 9091
	DevnetLocalAWMRelayerMetricsPort       = 9092
	FujiLocalAWMRelayerMetricsPort         = 9093

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

	UpgradeBytesFileName      = "upgrade.json"
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

	MetricsNetwork               = "network"
	MultiSig                     = "multi-sig"
	SkipUpdateFlag               = "skip-update-check"
	LastFileName                 = ".last_actions.json"
	APIRole                      = "API"
	ValidatorRole                = "Validator"
	MonitorRole                  = "Monitor"
	AWMRelayerRole               = "Relayer"
	LoadTestRole                 = "LoadTest"
	DefaultWalletCreationTimeout = 5 * time.Second

	DefaultConfirmTxTimeout = 20 * time.Second

	PayTxsFeesMsg = "pay transaction fees"

	CodespaceNameEnvVar = "CODESPACE_NAME"

	// E2E
	E2ENetworkPrefix        = "192.168.222"
	E2EListenPrefix         = "192.168.223"
	E2EClusterName          = "e2e"
	E2EDocker               = "docker"
	E2EDockerComposeFile    = "/tmp/avalanche-cli-docker-compose.yml"
	E2EDebugAvalanchegoPath = "E2E_AVALANCHEGO_PATH"
	GitExtension            = ".git"

	// Docker
	RemoteDockeSocketPath = "/var/run/docker.sock"

	// Avalanche InterChain Token Transfer
	ICTTDir     = "avalanche-interchain-token-transfer"
	ICTTURL     = "https://github.com/ava-labs/avalanche-interchain-token-transfer"
	ICTTBranch  = "main"
	ICTTVersion = "v1.0.0"

	// ICM
	DefaultTeleporterMessengerAddress      = "0x253b2784c75e510dD0fF1da844684a1aC0aa5fcf"
	MainnetCChainTeleporterRegistryAddress = "0x7C43605E14F391720e1b37E49C78C4b03A488d98"
	FujiCChainTeleporterRegistryAddress    = "0xF86Cb19Ad8405AEFa7d09C778215D2Cb6eBfB228"
)

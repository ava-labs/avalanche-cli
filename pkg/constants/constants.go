// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	"time"
)

const (
	DefaultPerms755        = 0o755
	WriteReadReadPerms     = 0o644
	WriteReadUserOnlyPerms = 0o600

	BaseDirName = ".avalanche-cli"
	LogDir      = "logs"

	ServerRunFile      = "gRPCserver.run"
	AvalancheCliBinDir = "bin"
	RunDir             = "runs"

	SuffixSeparator              = "_"
	SidecarFileName              = "sidecar.json"
	GenesisFileName              = "genesis.json"
	GenesisMainnetFileName       = "genesis_mainnet.json"
	ElasticSubnetConfigFileName  = "elastic_subnet_config.json"
	SidecarSuffix                = SuffixSeparator + SidecarFileName
	GenesisSuffix                = SuffixSeparator + GenesisFileName
	NodeFileName                 = "node.json"
	NodeCloudConfigFileName      = "node_cloud_config.json"
	TerraformDir                 = "terraform"
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

	ANRRequestTimeout   = 3 * time.Minute
	APIRequestTimeout   = 30 * time.Second
	FastGRPCDialTimeout = 100 * time.Millisecond

	SSHServerStartTimeout = 1 * time.Minute
	SSHScriptTimeout      = 2 * time.Minute
	SSHDirOpsTimeout      = 10 * time.Second
	SSHFileOpsTimeout     = 30 * time.Second
	SSHPOSTTimeout        = 10 * time.Second
	SSHSleepBetweenChecks = 1 * time.Second
	SSHScriptLogFilter    = "_AvalancheCLI_LOG_"
	SSHShell              = "/bin/bash"

	SimulatePublicNetwork = "SIMULATE_PUBLIC_NETWORK"

	FujiAPIEndpoint    = "https://api.avax-test.network"
	MainnetAPIEndpoint = "https://api.avax.network"

	// this depends on bootstrap snapshot
	LocalAPIEndpoint = "http://127.0.0.1:9650"
	LocalNetworkID   = 1337

	DevnetAPIEndpoint = ""
	DevnetNetworkID   = 1338

	DefaultTokenName = "TEST"

	HealthCheckInterval = 100 * time.Millisecond

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	SnapshotsDirName = "snapshots"

	DefaultSnapshotName = "default-1654102509"

	BootstrapSnapshotArchiveName = "bootstrapSnapshot.tar.gz"
	BootstrapSnapshotLocalPath   = "assets/" + BootstrapSnapshotArchiveName
	BootstrapSnapshotURL         = "https://github.com/ava-labs/avalanche-cli/raw/main/" + BootstrapSnapshotLocalPath
	BootstrapSnapshotSHA256URL   = "https://github.com/ava-labs/avalanche-cli/raw/main/assets/sha256sum.txt"

	BootstrapSnapshotSingleNodeArchiveName = "bootstrapSnapshotSingleNode.tar.gz"
	BootstrapSnapshotSingleNodeLocalPath   = "assets/" + BootstrapSnapshotSingleNodeArchiveName
	BootstrapSnapshotSingleNodeURL         = "https://github.com/ava-labs/avalanche-cli/raw/main/" + BootstrapSnapshotSingleNodeLocalPath
	BootstrapSnapshotSingleNodeSHA256URL   = "https://github.com/ava-labs/avalanche-cli/raw/main/assets/sha256sumSingleNode.txt"

	CliInstallationURL      = "https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh"
	ExpectedCliInstallErr   = "resource temporarily unavailable"
	EIPLimitErr             = "AddressLimitExceeded"
	ErrCreatingAWSNode      = "failed to create AWS Node"
	ErrCreatingGCPNode      = "failed to create GCP Node"
	ErrReleasingGCPStaticIP = "failed to release gcp static ip"
	KeyDir                  = "key"
	KeySuffix               = ".pk"
	YAMLSuffix              = ".yml"

	Enable = "enable"

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
	TerraformNodeConfigFile                      = "node_config.tf"
	AvalancheCLISuffix                           = "-avalanche-cli"
	AWSDefaultCredential                         = "default"
	GCPDefaultImageProvider                      = "ubuntu-os-cloud"
	GCPImageFilter                               = "family=ubuntu-2004* AND architecture=x86_64"
	GCPEnvVar                                    = "GOOGLE_APPLICATION_CREDENTIALS"
	GCPDefaultAuthKeyPath                        = "~/.config/gcloud/application_default_credentials.json"
	CertSuffix                                   = "-kp.pem"
	AWSSecurityGroupSuffix                       = "-sg"
	ExportSubnetSuffix                           = "-export.dat"
	SSHTCPPort                                   = 22
	AvalanchegoAPIPort                           = 9650
	AvalanchegoP2PPort                           = 9651
	CloudServerStorageSize                       = 1000
	OutboundPort                                 = 0
	Terraform                                    = "terraform"
	SetupCLIFromSourceBranch                     = "main"
	// Set this one to true while testing changes that alter CLI execution on cloud nodes
	// Disable it for releases to save cluster creation time
	EnableSetupCLIFromSource           = false
	BuildEnvGolangVersion              = "1.21.1"
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
	AnsibleSSHInventoryParams          = "-o StrictHostKeyChecking=no"
	AnsibleExtraVarsFlag               = "--extra-vars"

	ConfigAPMCredentialsFileKey  = "credentials-file"
	ConfigAPMAdminAPIEndpointKey = "admin-api-endpoint"
	ConfigNodeConfigKey          = "node-config"
	ConfigMetricsEnabledKey      = "MetricsEnabled"
	ConfigAutorizeCloudAccessKey = "AutorizeCloudAccess"
	ConfigSingleNodeEnabledKey   = "SingleNodeEnabled"
	OldConfigFileName            = ".avalanche-cli.json"
	OldMetricsConfigFileName     = ".avalanche-cli/config"
	DefaultConfigFileName        = ".avalanche-cli/config.json"

	AWSCloudService              = "Amazon Web Services"
	GCPCloudService              = "Google Cloud Platform"
	AnsibleSSHUser               = "ubuntu"
	AWSNodeAnsiblePrefix         = "aws_node"
	GCPNodeAnsiblePrefix         = "gcp_node"
	CustomVMDir                  = "vms"
	GCPStaticIPPrefix            = "static-ip"
	AvaLabsOrg                   = "ava-labs"
	AvalancheGoRepoName          = "avalanchego"
	SubnetEVMRepoName            = "subnet-evm"
	CliRepoName                  = "avalanche-cli"
	SubnetEVMReleaseURL          = "https://github.com/ava-labs/subnet-evm/releases/download/%s/%s"
	SubnetEVMArchive             = "subnet-evm_%s_linux_amd64.tar.gz"
	CloudNodeConfigBasePath      = "/home/ubuntu/.avalanchego/"
	CloudNodeSubnetEvmBinaryPath = "/home/ubuntu/.avalanchego/plugins/%s"
	CloudNodeStakingPath         = "/home/ubuntu/.avalanchego/staking/"
	CloudNodeConfigPath          = "/home/ubuntu/.avalanchego/configs/"
	CloudNodeCLIConfigBasePath   = "/home/ubuntu/.avalanche-cli/"

	AvalancheGoInstallDir = "avalanchego"
	SubnetEVMInstallDir   = "subnet-evm"

	SubnetEVMBin = "subnet-evm"

	DefaultNodeRunURL = "http://127.0.0.1:9650"

	APMDir                = ".apm"
	APMLogName            = "apm.log"
	DefaultAvaLabsPackage = "ava-labs/avalanche-plugins-core"
	APMPluginDir          = "apm_plugins"

	// #nosec G101
	GithubAPITokenEnvVarName = "AVALANCHE_CLI_GITHUB_TOKEN"

	ReposDir                   = "repos"
	SubnetDir                  = "subnets"
	NodesDir                   = "nodes"
	VMDir                      = "vms"
	ChainConfigDir             = "chains"
	AVMKeyName                 = "avm"
	EVMKeyName                 = "evm"
	PlatformKeyName            = "platform"
	SubnetType                 = "subnet type"
	PrecompileType             = "precompile type"
	CustomAirdrop              = "custom-airdrop"
	NumberOfAirdrops           = "airdrop-addresses"
	SubnetConfigFileName       = "subnet.json"
	ChainConfigFileName        = "chain.json"
	PerNodeChainConfigFileName = "per-node-chain.json"
	NodeConfigFileName         = "node-config.json"

	GitRepoCommitName  = "Avalanche-CLI"
	GitRepoCommitEmail = "info@avax.network"
	AvaLabsMaintainers = "ava-labs"

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

	Network        = "network"
	MultiSig       = "multi-sig"
	SkipUpdateFlag = "skip-update-check"
	LastFileName   = ".last_actions.json"

	DefaultWalletCreationTimeout = 5 * time.Second

	DefaultConfirmTxTimeout = 20 * time.Second

	PayTxsFeesMsg = "pay transaction fees"
)

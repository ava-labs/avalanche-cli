// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/spf13/cobra"
)

const (
	healthCheckPoolTime   = 10 * time.Second
	healthCheckTimeout    = 1 * time.Minute
	syncCheckPoolTime     = 10 * time.Second
	syncCheckTimeout      = 1 * time.Minute
	validateCheckPoolTime = 10 * time.Second
	validateCheckTimeout  = 1 * time.Minute
)

var (
	forceSubnetCreate   bool
	subnetGenesisFile   string
	useEvmSubnet        bool
	useCustomSubnet     bool
	evmVersion          string
	useLatestEvmVersion bool
	customVMRepoURL     string
	customVMBranch      string
	customVMBuildScript string
)

func newWizCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiz [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Creates a devnet together with a fully validated subnet into it.",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node wiz command creates a devnet and deploys, sync and validate a subnet into it. It creates the subnet if so needed.
`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         wiz,
	}
	cmd.Flags().BoolVar(&useStaticIP, "use-static-ip", true, "attach static Public IP on cloud servers")
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create node/s in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create node/s in GCP cloud")
	cmd.Flags().StringVar(&cmdLineRegion, "region", "", "create node/s in given region")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().IntVar(&numNodes, "num-nodes", 0, "number of nodes to create")
	cmd.Flags().StringVar(&cmdLineGCPCredentialsPath, "gcp-credentials", "", "use given GCP credentials")
	cmd.Flags().StringVar(&cmdLineGCPProjectName, "gcp-project", "", "use given GCP project")
	cmd.Flags().StringVar(&cmdLineAlternativeKeyPairName, "alternative-key-pair-name", "", "key pair name to use if default one generates conflicts")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	cmd.Flags().BoolVar(&defaultValidatorParams, "default-validator-params", false, "use default weight/start/duration params for subnet validator")

	cmd.Flags().BoolVar(&forceSubnetCreate, "force-subnet-create", false, "overwrite the existing subnet configuration if one exists")
	cmd.Flags().StringVar(&subnetGenesisFile, "subnet-genesis", "", "file path of the subnet genesis")
	cmd.Flags().BoolVar(&useEvmSubnet, "evm-subnet", false, "use Subnet-EVM as the subnet virtual machine")
	cmd.Flags().BoolVar(&useCustomSubnet, "custom-subnet", false, "use a custom VM as the subnet virtual machine")
	cmd.Flags().StringVar(&evmVersion, "evm-version", "", "version of Subnet-Evm to use")
	cmd.Flags().BoolVar(&useLatestEvmVersion, "latest-evm-version", false, "use latest Subnet-Evm version")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	return cmd
}

func wiz(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	exists, err := clusterExists(clusterName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cluster %s already exists", clusterName)
	}
	if !app.SidecarExists(subnetName) || forceSubnetCreate {
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
			useLatestEvmVersion,
			customVMRepoURL,
			customVMBranch,
			customVMBuildScript,
		); err != nil {
			return err
		}
	}
	return nil
	createDevnet = true
	useAvalanchegoVersionFromSubnet = subnetName
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Creating the devnet"))
	ux.Logger.PrintToUser("")
	err = createNodes(cmd, []string{clusterName})
	if err != nil {
		return err
	}
	if err := waitForHealthyCluster(clusterName, healthCheckTimeout, healthCheckPoolTime); err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Deploying the subnet"))
	ux.Logger.PrintToUser("")
	if err := deploySubnet(cmd, []string{clusterName, subnetName}); err != nil {
		return err
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
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	blockchainID := sc.Networks[models.Devnet.String()].BlockchainID
	if blockchainID == ids.Empty {
		return ErrNoBlockchainID
	}
	if err := waitForClusterSubnetStatus(clusterName, subnetName, blockchainID, status.Syncing, syncCheckTimeout, syncCheckPoolTime); err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Adding nodes as subnet validators"))
	ux.Logger.PrintToUser("")
	if err := validateSubnet(cmd, []string{clusterName, subnetName}); err != nil {
		return err
	}
	if err := waitForClusterSubnetStatus(clusterName, subnetName, blockchainID, status.Validating, validateCheckTimeout, validateCheckPoolTime); err != nil {
		return err
	}
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(logging.Green.Wrap("Devnet %s has been created and is validating subnet %s!"), clusterName, subnetName)
	return nil
}

func waitForHealthyCluster(
	clusterName string,
	timeout time.Duration,
	poolTime time.Duration,
) error {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Waiting for node(s) in cluster %s to be healthy...", clusterName)
	startTime := time.Now()
	for {
		notHealthyNodes, err := checkClusterIsHealthy(clusterName)
		if err != nil {
			return err
		}
		if len(notHealthyNodes) == 0 {
			ux.Logger.PrintToUser("Nodes healthy after %d seconds", uint32(time.Since(startTime).Seconds()))
			return nil
		}
		if time.Since(startTime) > timeout {
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Unhealthy Nodes")
			for _, failedNode := range notHealthyNodes {
				ux.Logger.PrintToUser("  " + failedNode)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("cluster not healthy after %d seconds", uint32(timeout.Seconds()))
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
	if err := app.CreateAnsibleStatusDir(); err != nil {
		return err
	}
	defer func() {
		_ = app.RemoveAnsibleStatusDir()
	}()
	ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	startTime := time.Now()
	for {
		if err := ansible.RunAnsiblePlaybookSubnetSyncStatus(
			app.GetAnsibleDir(),
			app.GetSubnetSyncJSONFile(),
			blockchainID.String(),
			app.GetAnsibleInventoryDirPath(clusterName),
			"all",
		); err != nil {
			return err
		}
		failedNodes := []string{}
		for _, ansibleHostID := range ansibleHostIDs {
			subnetSyncStatus, err := parseSubnetSyncOutput(app.GetSubnetSyncJSONFile() + "." + ansibleHostID)
			if err != nil {
				return err
			}
			if subnetSyncStatus != targetStatus.String() {
				failedNodes = append(failedNodes, ansibleHostID)
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

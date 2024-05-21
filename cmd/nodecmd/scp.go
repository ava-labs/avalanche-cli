// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	isRecursive     bool
	withCompression bool
)

type ClusterOp int64

const (
	noCluster ClusterOp = iota
	srcCluster
	dstCluster
)

func newSCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scp SOURCE DEST",
		Short: "(ALPHA Warning) Securely copy files to and from nodes",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node scp command securely copies files to and from nodes. Remote source or destionation can be specified using the following format:
[clusterName|nodeID|instanceID|IP]:/path/to/file. Regular expressions are supported for the source files like /tmp/*.txt.
File transfer to the nodes are parallelized. IF source or destination is cluster, the other should be a local file path. 
If both destinations are remote, they must be nodes for the same cluster and not clusters themselves.
For example:
$ avalanche node scp [cluster1|node1]:/tmp/file.txt /tmp/file.txt
$ avalanche node scp /tmp/file.txt [cluster1|NodeID-XXXX]:/tmp/file.txt
$ avalanche node scp node1:/tmp/file.txt NodeID-XXXX:/tmp/file.txt
`,
		Args: cobrautils.MinimumNArgs(2),
		RunE: scpNode,
	}
	cmd.Flags().BoolVar(&isRecursive, "recursive", false, "copy directories recursively")
	cmd.Flags().BoolVar(&withCompression, "compress", false, "use compression for ssh")
	cmd.Flags().BoolVar(&includeMonitor, "with-monitor", false, "include monitoring node for scp cluster operations")
	cmd.Flags().BoolVar(&includeLoadTest, "with-loadtest", false, "include loadtest node for scp cluster operations")
	return cmd
}

func scpNode(_ *cobra.Command, args []string) error {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if len(clustersConfig.Clusters) == 0 {
		ux.Logger.PrintToUser("There are no clusters defined.")
		return nil
	}

	sourcePath, destPath := args[0], args[1]
	sourceClusterNameOrNodeID, sourcePath := utils.SplitSCPPath(sourcePath)
	destClusterNameOrNodeID, destPath := utils.SplitSCPPath(destPath)

	// check if source and destination are both clusters
	sourceClusterExists, err := checkClusterExists(sourceClusterNameOrNodeID)
	if err != nil {
		return err
	}
	destClusterExists, err := checkClusterExists(destClusterNameOrNodeID)
	if err != nil {
		return err
	}
	if sourceClusterExists && destClusterExists {
		return fmt.Errorf("both source and destination cannot be clusters")
	}

	switch {
	case sourceClusterExists:
		// source is a cluster
		clusterName := sourceClusterNameOrNodeID
		clusterHosts, err := GetAllClusterHosts(clusterName)
		if err != nil {
			return err
		}
		return scpHosts(srcCluster, clusterHosts, sourcePath, utils.CombineScpPath(destClusterNameOrNodeID, destPath), clusterName, true)
	case destClusterExists:
		// destination is a cluster
		clusterName := destClusterNameOrNodeID
		clusterHosts, err := GetAllClusterHosts(clusterName)
		if err != nil {
			return err
		}
		return scpHosts(dstCluster, clusterHosts, utils.CombineScpPath(sourceClusterNameOrNodeID, sourcePath), destPath, clusterName, false)
	default:
		if sourceClusterNameOrNodeID == destClusterNameOrNodeID {
			return fmt.Errorf("source and destination cannot be the same node")
		}
		// source is remote
		srcPath := utils.CombineScpPath(sourceClusterNameOrNodeID, sourcePath)
		dstPath := utils.CombineScpPath(destClusterNameOrNodeID, destPath)
		ux.Logger.Info("scp src %s dst %s", srcPath, dstPath)
		if sourceClusterNameOrNodeID != "" {
			selectedHost, clusterName := getHostClusterPair(sourceClusterNameOrNodeID)
			if selectedHost != nil && clusterName != "" {
				return scpHosts(noCluster, []*models.Host{selectedHost}, srcPath, dstPath, clusterName, false)
			}
		} else if destClusterNameOrNodeID != "" {
			selectedHost, clusterName := getHostClusterPair(destClusterNameOrNodeID)
			if selectedHost != nil && clusterName != "" {
				return scpHosts(noCluster, []*models.Host{selectedHost}, srcPath, dstPath, clusterName, false)
			}
		}
		return fmt.Errorf("source or destination not found")
	}
}

// scpHosts securely copies files to and from nodes.
func scpHosts(op ClusterOp, hosts []*models.Host, sourcePath, destPath string, clusterName string, separateNodeFolder bool) error {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	spinSession := ux.NewUserSpinner()
	for _, host := range hosts {
		// prepare both source and destination for scp command
		scpPrefix, err := prepareSCPTarget(op, host, clusterName, sourcePath, true)
		if err != nil {
			return err
		}
		scpSuffix, err := prepareSCPTarget(op, host, clusterName, destPath, false)
		if err != nil {
			return err
		}
		prefixIP, prefixPath := utils.SplitSCPPath(scpPrefix)
		suffixIP, suffixPath := utils.SplitSCPPath(scpSuffix)
		switch op {
		case srcCluster:
			prefixIP = host.IP
			// skip the same host
			if suffixIP == host.IP {
				continue
			}
		case dstCluster:
			suffixIP = host.IP
			// skip the same host
			if prefixIP == host.IP {
				continue
			}
		default:
			// noCluster
		}
		if separateNodeFolder {
			// add nodeID and clusterName to destination path if source is cluster, i.e. multiple nodes
			suffixPath = fmt.Sprintf("%s/%s_%s/", suffixPath, clusterName, host.NodeID)
		}
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			spinner := spinSession.SpinToUser(fmt.Sprintf("[%s] transferring file(s)", host.GetCloudID()))
			scpCmd := ""
			scpCmd, err = utils.GetSCPCommandString(
				host.SSHPrivateKeyPath,
				prefixIP,
				prefixPath,
				suffixIP,
				suffixPath,
				isRecursive,
				withCompression)
			if err != nil {
				ux.SpinFailWithError(spinner, "", err)
				nodeResults.AddResult(host.NodeID, "", err)
				return
			}
			ux.Logger.Info("About to execute scp command: %s", scpCmd)
			cmd := utils.Command(scpCmd)

			if cmdOut, err := cmd.Output(); err != nil {
				ux.SpinFailWithError(spinner, string(cmdOut), err)
				nodeResults.AddResult(host.NodeID, string(cmdOut), err)
			} else {
				ux.SpinComplete(spinner)
				nodeResults.AddResult(host.NodeID, "", nil)
			}
		}(&wgResults, host)
	}
	wg.Wait()
	spinSession.Stop()
	if wgResults.HasErrors() {
		return fmt.Errorf("failed to scp for node(s) %s", wgResults.GetErrorHostMap())
	}
	return nil
}

// prepareSCPTarget prepares the target for scp command
func prepareSCPTarget(op ClusterOp, host *models.Host, clusterName string, dest string, isSrc bool) (string, error) {
	// valid clusterName - is already checked
	if !strings.Contains(dest, ":") {
		// destination is local, ready to go
		return dest, nil
	}
	// destination is remote
	splitDest := strings.Split(dest, ":")
	node := splitDest[0]
	path := splitDest[1]
	if utils.IsValidIP(node) {
		// destination is IP, ready to go
		return dest, nil
	}
	// destination is cloudID or NodeID. clusterName is already checked and valid
	clusterHosts, err := GetAllClusterHosts(clusterName)
	if err != nil {
		return "", err
	}
	selectedHost := utils.Filter(clusterHosts, func(h *models.Host) bool {
		_, cloudHostID, _ := models.HostAnsibleIDToCloudID(h.NodeID)
		hostNodeID, _ := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
		return h.GetCloudID() == node || hostNodeID.String() == node || h.IP == node
	})
	switch {
	case len(selectedHost) == 0:
		return "", fmt.Errorf("node %s not found in cluster %s", node, clusterName)
	case len(selectedHost) > 2:
		return "", fmt.Errorf("more then 1 node found for %s in cluster %s", node, clusterName)
	case (op == srcCluster && isSrc) || (op == dstCluster && !isSrc):
		return fmt.Sprintf("%s:%s", host.IP, path), nil
	default:
		return fmt.Sprintf("%s:%s", selectedHost[0].IP, path), nil
	}
}

// getHostClusterPair returns the host and cluster name for the given node or cloudID
func getHostClusterPair(nodeOrCloudIDOrIP string) (*models.Host, string) {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return nil, ""
		}
	}
	for clusterName := range clustersConfig.Clusters {
		clusterHosts, err := GetAllClusterHosts(clusterName)
		if err != nil {
			return nil, ""
		}
		selectedHost := utils.Filter(clusterHosts, func(h *models.Host) bool {
			_, cloudHostID, _ := models.HostAnsibleIDToCloudID(h.NodeID)
			hostNodeID, _ := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
			return h.GetCloudID() == nodeOrCloudIDOrIP || hostNodeID.String() == nodeOrCloudIDOrIP || h.IP == nodeOrCloudIDOrIP
		})
		switch {
		case len(selectedHost) == 0:
			continue
		case len(selectedHost) > 2:
			return nil, ""
		default:
			return selectedHost[0], clusterName
		}
	}
	return nil, ""
}

/*
func getFileSizeInKB(path string) (int64, error) {
	if !strings.Contains(path, ":") {
		// path is local
		return utils.SizeInKB(path)
	} else {
		//path is remote
		splitDest := strings.Split(path, ":")
		node := splitDest[0]
		path := splitDest[1]
		if utils.IsValidIP(node) {
			selectedHost, clusterName := getHostClusterPair(node)
			if selectedHost != nil && clusterName != "" {
				duOutput, err := selectedHost.Command(fmt.Sprintf("du -sk %s", path), nil, constants.SSHLongRunningScriptTimeout)
				if err != nil {
					return 0, err
				}
				return strconv.ParseInt(strings.Split(string(duOutput), " ")[0], 10, 64)
			}
		}
	}
	return 0, fmt.Errorf("failed to get file size for %s", path)
}
*/
